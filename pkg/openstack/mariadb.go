package openstack

import (
	"context"
	"fmt"

	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	mariadbv1 "github.com/openstack-k8s-operators/mariadb-operator/api/v1beta1"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ReconcileMariaDB -
func ReconcileMariaDB(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, helper *helper.Helper) (ctrl.Result, error) {
	if !instance.Spec.Mariadb.Enabled {
		return ctrl.Result{}, nil
	}

	mariadb := &mariadbv1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "openstack", //FIXME
			Namespace: instance.Namespace,
		},
	}

	helper.GetLogger().Info("Reconciling MariaDB", "MariaDB.Namespace", instance.Namespace, "mariadb.Name", "openstack")
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), mariadb, func() error {
		instance.Spec.Mariadb.Template.DeepCopyInto(&mariadb.Spec)
		if mariadb.Spec.Secret == "" {
			mariadb.Spec.Secret = instance.Spec.Secret
		}
		if mariadb.Spec.StorageClass == "" {
			mariadb.Spec.StorageClass = instance.Spec.StorageClass
		}
		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), mariadb, helper.GetScheme())
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneMariaDBReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackControlPlaneMariaDBReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		helper.GetLogger().Info(fmt.Sprintf("MariaDB %s - %s", mariadb.Name, op))
	}

	if mariadb.IsReady() {
		instance.Status.Conditions.MarkTrue(corev1beta1.OpenStackControlPlaneMariaDBReadyCondition, corev1beta1.OpenStackControlPlaneMariaDBReadyMessage)
	} else {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneMariaDBReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			corev1beta1.OpenStackControlPlaneMariaDBReadyRunningMessage))
	}

	return ctrl.Result{}, nil

}

// DeleteMariaDB -
func DeleteMariaDB(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, helper *helper.Helper) (ctrl.Result, error) {
	overallRes := ctrl.Result{}

	mariaDBList := &mariadbv1.MariaDBList{}

	if err := helper.GetClient().List(context.Background(), mariaDBList); err != nil {
		return overallRes, err
	}

	for _, mariaDB := range mariaDBList.Items {
		res, err := checkDeleteSubresource(ctx, instance, helper, &mariaDB)

		if err != nil {
			return res, err
		} else if (res != ctrl.Result{}) {
			overallRes = res
		}
	}

	return overallRes, nil
}
