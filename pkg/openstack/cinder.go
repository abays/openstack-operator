package openstack

import (
	"context"
	"fmt"

	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	cinderv1 "github.com/openstack-k8s-operators/cinder-operator/api/v1beta1"
	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ReconcileCinder -
func ReconcileCinder(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, helper *helper.Helper) (ctrl.Result, error) {
	if !instance.Spec.Cinder.Enabled {
		return ctrl.Result{}, nil
	}

	cinder := &cinderv1.Cinder{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cinder",
			Namespace: instance.Namespace,
		},
	}

	helper.GetLogger().Info("Reconciling Cinder", "Cinder.Namespace", instance.Namespace, "Cinder.Name", "cinder")
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), cinder, func() error {
		instance.Spec.Cinder.Template.DeepCopyInto(&cinder.Spec)
		if cinder.Spec.Secret == "" {
			cinder.Spec.Secret = instance.Spec.Secret
		}
		//if cinder.Spec.NodeSelector == nil && instance.Spec.NodeSelector != nil {
		//cinder.Spec.NodeSelector = instance.Spec.NodeSelector
		//}
		if cinder.Spec.DatabaseInstance == "" {
			//cinder.Spec.DatabaseInstance = instance.Name // name of MariaDB we create here
			cinder.Spec.DatabaseInstance = "openstack" //FIXME: see above
		}
		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), cinder, helper.GetScheme())
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneCinderReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackControlPlaneCinderReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		helper.GetLogger().Info(fmt.Sprintf("Cinder %s - %s", cinder.Name, op))
	}

	// TODO add once rabbitmq transportURL is integrated with Cinder
	// if cinder.IsReady() {
	// 	instance.Status.Conditions.MarkTrue(corev1beta1.OpenStackControlPlaneCinderReadyCondition, corev1beta1.OpenStackControlPlaneCinderReadyMessage)
	// } else {
	// 	instance.Status.Conditions.Set(condition.FalseCondition(
	// 		corev1beta1.OpenStackControlPlaneCinderReadyCondition,
	// 		condition.RequestedReason,
	// 		condition.SeverityInfo,
	// 		corev1beta1.OpenStackControlPlaneCinderReadyRunningMessage))
	// }

	return ctrl.Result{}, nil

}

// DeleteCinder -
func DeleteCinder(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, helper *helper.Helper) (ctrl.Result, error) {
	overallRes := ctrl.Result{}

	cinderList := &cinderv1.CinderList{}

	if err := helper.GetClient().List(context.Background(), cinderList); err != nil {
		return overallRes, err
	}

	for _, cinder := range cinderList.Items {
		res, err := checkDeleteSubresource(ctx, instance, helper, &cinder)

		if err != nil {
			return res, err
		} else if (res != ctrl.Result{}) {
			overallRes = res
		}
	}

	return overallRes, nil
}
