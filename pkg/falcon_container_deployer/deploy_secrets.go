package falcon_container_deployer

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crowdstrike/falcon-operator/pkg/k8s_utils"
	"github.com/crowdstrike/falcon-operator/pkg/registry/pulltoken"
)

const (
	SECRET_NAME              = "crowdstrike-falcon-pull-secret"
	SECRET_LABEL             = "crowdstrike.com/provider"
	SECRET_LABEL_VALUE       = "crowdstrike"
	INJECTION_LABEL          = "sensor.falcon-system.crowdstrike.com/injection"
	INJECTION_LABEL_DISABLED = "disabled"
)

func (d *FalconContainerDeployer) UpsertCrowdStrikeSecrets() error {
	namespaces, err := d.namespacesMissingSecrets()
	if err != nil || len(namespaces) == 0 {
		return err
	}

	pulltoken, err := pulltoken.CrowdStrike(d.Ctx, d.falconApiConfig())
	if err != nil {
		return err
	}

	for ns := range namespaces {
		err = d.createCrowdstrikeSecret(ns, pulltoken)
		if err != nil && ns == "falcon-system-configure" {
			return err
		}
	}

	return nil
}

func (d *FalconContainerDeployer) createCrowdstrikeSecret(namespace string, pulltoken []byte) error {
	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      SECRET_NAME,
			Namespace: namespace,
			Labels: map[string]string{
				SECRET_LABEL: SECRET_LABEL_VALUE,
			},
		},
		Data: map[string][]byte{
			".dockerconfigjson": pulltoken,
		},
		Type: corev1.SecretTypeDockerConfigJson,
	}
	err := ctrl.SetControllerReference(d.Instance, secret, d.Scheme)
	if err != nil {
		d.Log.Error(err, "Unable to assign Controller Reference to the Pull Secret")
	}
	err = d.Client.Create(d.Ctx, secret)
	if err != nil {
		if !errors.IsAlreadyExists(err) {
			d.Log.Error(err, "Failed to schedule new Pull Secret", "Secret.Namespace", namespace, "Secret.Name", SECRET_NAME)
			return err
		}
	} else {
		d.Log.Info("Created a new Pull Secret", "Secret.Namespace", namespace, "Secret.Name", SECRET_NAME)
	}
	return nil
}

func (d *FalconContainerDeployer) namespacesMissingSecrets() (map[string]void, error) {
	nsMap := map[string]void{}
	nsList := &corev1.NamespaceList{}
	if err := d.Client.List(d.Ctx, nsList); err != nil {
		return nil, err
	}
	for _, ns := range nsList.Items {
		if ns.Name == "default" || ns.Name == "kube-system" {
			continue
		}
		if ns.Annotations != nil && ns.Annotations[INJECTION_LABEL] == INJECTION_LABEL_DISABLED {
			continue
		}
		nsMap[ns.Name] = void{}
	}

	secretList, err := d.listCrowdStrikeSecrets()
	if err != nil {
		return nil, err
	}

	for _, secret := range secretList.Items {
		delete(nsMap, secret.Namespace)
	}

	return nsMap, nil

}

func (d *FalconContainerDeployer) listCrowdStrikeSecrets() (*corev1.SecretList, error) {
	return k8s_utils.QuerySecrets(d.Client, client.MatchingLabels(map[string]string{SECRET_LABEL: SECRET_LABEL_VALUE}))(d.Ctx)
}

type void struct{}
