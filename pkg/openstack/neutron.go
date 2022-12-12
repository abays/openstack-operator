package openstack

import (
	"context"
	"fmt"

	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	neutronv1 "github.com/openstack-k8s-operators/neutron-operator/api/v1beta1"
	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ReconcileNeutron -
func ReconcileNeutron(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, helper *helper.Helper) (ctrl.Result, error) {
	if !instance.Spec.Neutron.Enabled {
		return ctrl.Result{}, nil
	}

	neutronAPI := &neutronv1.NeutronAPI{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "neutron",
			Namespace: instance.Namespace,
		},
	}

	helper.GetLogger().Info("Reconciling neutronAPI", "neutronAPI.Namespace", instance.Namespace, "neutronAPI.Name", "neutron")
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), neutronAPI, func() error {
		instance.Spec.Neutron.Template.DeepCopyInto(&neutronAPI.Spec)
		if neutronAPI.Spec.Secret == "" {
			neutronAPI.Spec.Secret = instance.Spec.Secret
		}
		if neutronAPI.Spec.NodeSelector == nil && instance.Spec.NodeSelector != nil {
			neutronAPI.Spec.NodeSelector = instance.Spec.NodeSelector
		}
		if neutronAPI.Spec.DatabaseInstance == "" {
			neutronAPI.Spec.DatabaseInstance = "openstack"
		}
		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), neutronAPI, helper.GetScheme())
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneNeutronReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackControlPlaneNeutronReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		helper.GetLogger().Info(fmt.Sprintf("neutronAPI %s - %s", neutronAPI.Name, op))
	}

	if neutronAPI.IsReady() {
		instance.Status.Conditions.MarkTrue(corev1beta1.OpenStackControlPlaneNeutronReadyCondition, corev1beta1.OpenStackControlPlaneNeutronReadyMessage)
	} else {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneNeutronReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			corev1beta1.OpenStackControlPlaneNeutronReadyRunningMessage))
	}

	return ctrl.Result{}, nil

}

// DeleteNeutron -
func DeleteNeutron(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, helper *helper.Helper) (ctrl.Result, error) {
	overallRes := ctrl.Result{}

	neutronAPIList := &neutronv1.NeutronAPIList{}

	if err := helper.GetClient().List(context.Background(), neutronAPIList); err != nil {
		return overallRes, err
	}

	for _, neutronAPI := range neutronAPIList.Items {
		res, err := checkDeleteSubresource(ctx, instance, helper, &neutronAPI)

		if err != nil {
			return res, err
		} else if (res != ctrl.Result{}) {
			overallRes = res
		}
	}

	return overallRes, nil
}
