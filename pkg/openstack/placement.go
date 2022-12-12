package openstack

import (
	"context"
	"fmt"

	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	placementv1 "github.com/openstack-k8s-operators/placement-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ReconcilePlacementAPI -
func ReconcilePlacementAPI(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, helper *helper.Helper) (ctrl.Result, error) {
	if !instance.Spec.Placement.Enabled {
		return ctrl.Result{}, nil
	}

	placementAPI := &placementv1.PlacementAPI{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "placement",
			Namespace: instance.Namespace,
		},
	}

	helper.GetLogger().Info("Reconciling placementAPI", "placementAPI.Namespace", instance.Namespace, "placementAPI.Name", "placement")
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), placementAPI, func() error {
		instance.Spec.Placement.Template.DeepCopyInto(&placementAPI.Spec)
		if placementAPI.Spec.Secret == "" {
			placementAPI.Spec.Secret = instance.Spec.Secret
		}
		if placementAPI.Spec.NodeSelector == nil && instance.Spec.NodeSelector != nil {
			placementAPI.Spec.NodeSelector = instance.Spec.NodeSelector
		}
		if placementAPI.Spec.DatabaseInstance == "" {
			placementAPI.Spec.DatabaseInstance = "openstack"
		}
		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), placementAPI, helper.GetScheme())
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlanePlacementAPIReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackControlPlanePlacementAPIReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		helper.GetLogger().Info(fmt.Sprintf("placementAPI %s - %s", placementAPI.Name, op))
	}

	if placementAPI.IsReady() {
		instance.Status.Conditions.MarkTrue(corev1beta1.OpenStackControlPlanePlacementAPIReadyCondition, corev1beta1.OpenStackControlPlanePlacementAPIReadyMessage)
	} else {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlanePlacementAPIReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			corev1beta1.OpenStackControlPlanePlacementAPIReadyRunningMessage))
	}

	return ctrl.Result{}, nil

}

// DeletePlacement -
func DeletePlacement(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, helper *helper.Helper) (ctrl.Result, error) {
	overallRes := ctrl.Result{}

	placementAPIList := &placementv1.PlacementAPIList{}

	if err := helper.GetClient().List(context.Background(), placementAPIList); err != nil {
		return overallRes, err
	}

	for _, placementAPI := range placementAPIList.Items {
		res, err := checkDeleteSubresource(ctx, instance, helper, &placementAPI)

		if err != nil {
			return res, err
		} else if (res != ctrl.Result{}) {
			overallRes = res
		}
	}

	return overallRes, nil
}
