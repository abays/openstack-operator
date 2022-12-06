package openstack

import (
	"context"
	"fmt"

	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	ovnv1 "github.com/openstack-k8s-operators/ovn-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ReconcileOVN -
func ReconcileOVN(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, helper *helper.Helper) (ctrl.Result, error) {
	if !instance.Spec.Ovn.Enabled {
		return ctrl.Result{}, nil
	}
	for name, dbcluster := range instance.Spec.Ovn.Template.OVNDBCluster {
		OVNDBCluster := &ovnv1.OVNDBCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: instance.Namespace,
			},
		}

		helper.GetLogger().Info("Reconciling OVNDBCluster", "OVNDBCluster.Namespace", instance.Namespace, "OVNDBCluster.Name", name)
		op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), OVNDBCluster, func() error {

			dbcluster.DeepCopyInto(&OVNDBCluster.Spec)

			if OVNDBCluster.Spec.NodeSelector == nil && instance.Spec.NodeSelector != nil {
				OVNDBCluster.Spec.NodeSelector = instance.Spec.NodeSelector
			}
			if OVNDBCluster.Spec.StorageClass == "" {
				OVNDBCluster.Spec.StorageClass = instance.Spec.StorageClass
			}

			err := controllerutil.SetControllerReference(helper.GetBeforeObject(), OVNDBCluster, helper.GetScheme())
			if err != nil {
				return err
			}
			return nil
		})

		if err != nil {
			instance.Status.Conditions.Set(condition.FalseCondition(
				corev1beta1.OpenStackControlPlaneOVNReadyCondition,
				condition.ErrorReason,
				condition.SeverityWarning,
				corev1beta1.OpenStackControlPlaneOVNReadyErrorMessage,
				err.Error()))
			return ctrl.Result{}, err
		}
		if op != controllerutil.OperationResultNone {
			helper.GetLogger().Info(fmt.Sprintf("OVNDBCluster %s - %s", OVNDBCluster.Name, op))
		}

	}

	OVNNorthd := &ovnv1.OVNNorthd{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ovnnorthd",
			Namespace: instance.Namespace,
		},
	}

	helper.GetLogger().Info("Reconciling OVNNorthd", "OVNNorthd.Namespace", instance.Namespace, "OVNNorthd.Name", "ovnnorthd")
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), OVNNorthd, func() error {

		instance.Spec.Ovn.Template.OVNNorthd.DeepCopyInto(&OVNNorthd.Spec)

		if OVNNorthd.Spec.NodeSelector == nil && instance.Spec.NodeSelector != nil {
			OVNNorthd.Spec.NodeSelector = instance.Spec.NodeSelector
		}

		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), OVNNorthd, helper.GetScheme())
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneOVNReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackControlPlaneOVNReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		helper.GetLogger().Info(fmt.Sprintf("OVNNorthd %s - %s", OVNNorthd.Name, op))
	}

	if OVNNorthd.IsReady() {
		instance.Status.Conditions.MarkTrue(corev1beta1.OpenStackControlPlaneOVNReadyCondition, corev1beta1.OpenStackControlPlaneOVNReadyMessage)
	} else {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneOVNReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			corev1beta1.OpenStackControlPlaneOVNReadyRunningMessage))
	}
	return ctrl.Result{}, nil
}

// DeleteOVN -
func DeleteOVN(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, helper *helper.Helper) (ctrl.Result, error) {
	overallRes := ctrl.Result{}

	ovnDBClusterList := &ovnv1.OVNDBClusterList{}

	if err := helper.GetClient().List(context.Background(), ovnDBClusterList); err != nil {
		return overallRes, err
	}

	for _, ovnDBCluster := range ovnDBClusterList.Items {
		res, err := checkDeleteSubresource(ctx, instance, helper, &ovnDBCluster)

		if err != nil {
			return res, err
		} else if (res != ctrl.Result{}) {
			overallRes = res
		}
	}

	ovnNorthdList := &ovnv1.OVNNorthdList{}

	if err := helper.GetClient().List(context.Background(), ovnNorthdList); err != nil {
		return overallRes, err
	}

	for _, ovnNorthd := range ovnNorthdList.Items {
		res, err := checkDeleteSubresource(ctx, instance, helper, &ovnNorthd)

		if err != nil {
			return res, err
		} else if (res != ctrl.Result{}) {
			overallRes = res
		}
	}

	return overallRes, nil
}
