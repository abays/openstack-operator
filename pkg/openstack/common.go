package openstack

import (
	"context"
	"fmt"
	"time"

	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// checkDeleteSubresource -
func checkDeleteSubresource(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, helper *helper.Helper, subRes client.Object) (ctrl.Result, error) {
	found := false
	res := ctrl.Result{}

	for _, ownerRef := range subRes.GetOwnerReferences() {
		if ownerRef.Kind == instance.Kind && ownerRef.Name == instance.Name {
			if subRes.GetDeletionTimestamp().IsZero() {
				if err := helper.GetClient().Delete(ctx, subRes); err != nil {
					return res, err
				}
			}

			found = true
		}
	}

	if found {
		res = ctrl.Result{RequeueAfter: time.Duration(5) * time.Second}
		helper.GetLogger().Info(fmt.Sprintf("OpenStackControlPlane %s deletion waiting for deletion of %s %s", instance.Name, subRes.GetObjectKind().GroupVersionKind().Kind, subRes.GetName()))
	}

	return res, nil
}
