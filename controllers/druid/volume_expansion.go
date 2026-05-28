/*
Licensed to the Apache Software Foundation (ASF) under one
or more contributor license agreements.  See the NOTICE file
distributed with this work for additional information
regarding copyright ownership.  The ASF licenses this file
to you under the Apache License, Version 2.0 (the
"License"); you may not use this file except in compliance
with the License.  You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing,
software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
KIND, either express or implied.  See the License for the
specific language governing permissions and limitations
under the License.
*/
package druid

import (
	"context"
	"errors"
	"fmt"

	"github.com/apache/druid-operator/apis/druid/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	storage "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func validateVolumeClaimTemplateSpec(drd *v1alpha1.Druid) error {
	for _, nodeSpec := range drd.Spec.Nodes {
		if nodeSpec.Kind == "StatefulSet" {
			if err := validateNodeVolumeClaimTemplateSpec(&nodeSpec); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateNodeVolumeClaimTemplateSpec(nodeSpec *v1alpha1.DruidNodeSpec) error {
	for _, vct := range nodeSpec.VolumeClaimTemplates {
		if vct.Spec.StorageClassName == nil || *vct.Spec.StorageClassName == "" {
			return fmt.Errorf("node group %s has volume claim template without storage class which is not allowed: %s",
				nodeSpec.NodeType, vct.Name)
		}
	}
	return nil
}

func expandStatefulSetVolumes(ctx context.Context, sdk client.Client, m *v1alpha1.Druid,
	nodeSpec *v1alpha1.DruidNodeSpec, emitEvent EventEmitter, nodeSpecUniqueStr string) error {

	isEnabled, err := isVolumeExpansionEnabled(ctx, sdk, m, nodeSpec, emitEvent)
	if err != nil {
		return err
	}

	if isEnabled {
		err := scalePVCForSts(ctx, sdk, nodeSpec, nodeSpecUniqueStr, m, emitEvent)
		if err != nil {
			return err
		}
	}

	return nil
}

func isVolumeExpansionEnabled(ctx context.Context, sdk client.Client, m *v1alpha1.Druid, nodeSpec *v1alpha1.DruidNodeSpec, emitEvent EventEmitter) (bool, error) {

	for _, nodeVCT := range nodeSpec.VolumeClaimTemplates {
		if nodeVCT.Spec.StorageClassName == nil {
			err := errors.New("StorageClassName does not exists")
			logger.WithValues("NodeType", nodeSpec.NodeType, "VolumeClaimTemplate", nodeVCT.Name).
				Error(err, "storageClassName does not exists in spec")
			return false, err
		}
		sc, err := readers.Get(ctx, sdk, *nodeVCT.Spec.StorageClassName, m, func() object { return &storage.StorageClass{} }, emitEvent)
		if err != nil {
			return false, err
		}

		allowExpansion := sc.(*storage.StorageClass).AllowVolumeExpansion
		if allowExpansion != nil && *allowExpansion {
			return true, nil
		}
	}
	return false, nil
}

// scalePVCForSts shall expand the StatefulSet's VolumeClaimTemplates size as well as N no of pvc supported by the sts.
// PVCs are correlated to their VCTs by name to avoid cross-VCT size mismatches.
func scalePVCForSts(ctx context.Context, sdk client.Client, nodeSpec *v1alpha1.DruidNodeSpec, nodeSpecUniqueStr string, drd *v1alpha1.Druid, emitEvent EventEmitter) error {

	getSTSList, err := readers.List(ctx, sdk, drd, makeLabelsForDruid(drd), emitEvent, func() objectList { return &appsv1.StatefulSetList{} }, func(listObj runtime.Object) []object {
		items := listObj.(*appsv1.StatefulSetList).Items
		result := make([]object, len(items))
		for i := 0; i < len(items); i++ {
			result[i] = &items[i]
		}
		return result
	})
	if err != nil {
		return nil
	}

	// Dont proceed unless all statefulsets are up and running.
	//  This can cause the go routine to panic
	for _, sts := range getSTSList {
		if sts.(*appsv1.StatefulSet).Status.Replicas != sts.(*appsv1.StatefulSet).Status.ReadyReplicas {
			return nil
		}
	}

	// return nil, in case return err the program halts since sts would not be able
	// we would like the operator to create sts.
	stsObj, err := readers.Get(ctx, sdk, nodeSpecUniqueStr, drd, func() object { return &appsv1.StatefulSet{} }, emitEvent)
	if err != nil {
		return nil
	}

	statefulSet := stsObj.(*appsv1.StatefulSet)

	pvcLabels := map[string]string{
		"nodeSpecUniqueStr": nodeSpecUniqueStr,
	}

	pvcList, err := readers.List(ctx, sdk, drd, pvcLabels, emitEvent, func() objectList { return &v1.PersistentVolumeClaimList{} }, func(listObj runtime.Object) []object {
		items := listObj.(*v1.PersistentVolumeClaimList).Items
		result := make([]object, len(items))
		for i := 0; i < len(items); i++ {
			result[i] = &items[i]
		}
		return result
	})
	if err != nil {
		return nil
	}

	// Group PVCs by their VCT name so we only patch PVCs belonging to the correct VCT
	pvcsByVCT := make(map[string][]*v1.PersistentVolumeClaim)
	for _, pvcObj := range pvcList {
		pvc := pvcObj.(*v1.PersistentVolumeClaim)
		vctName := extractVCTNameFromPVC(pvc.Name, statefulSet.Name)
		if vctName != "" {
			pvcsByVCT[vctName] = append(pvcsByVCT[vctName], pvc)
		}
	}

	// Build map of current STS VCTs by name
	currentVCTMap := make(map[string]v1.PersistentVolumeClaim)
	for _, vct := range statefulSet.Spec.VolumeClaimTemplates {
		currentVCTMap[vct.Name] = vct
	}

	stsDeleted := false

	for _, desiredVCT := range nodeSpec.VolumeClaimTemplates {
		currentVCT, exists := currentVCTMap[desiredVCT.Name]
		if !exists {
			continue
		}

		desiredSize := desiredVCT.Spec.Resources.Requests[v1.ResourceStorage]
		currentSize := currentVCT.Spec.Resources.Requests[v1.ResourceStorage]

		// Use Cmp() instead of AsInt64() because AsInt64() fails for quantities
		// with non-integer values like "2.2Ti" which are stored as milli-units.
		desiredVsCurrent := desiredSize.Cmp(currentSize)

		if desiredVsCurrent < 0 {
			e := fmt.Errorf("shrinking of sts pvc size [sts:%s] in [namespace:%s] for VCT [%s] is not supported",
				statefulSet.Name, statefulSet.Namespace, desiredVCT.Name)
			logger.Error(e, e.Error(), "name", drd.Name, "namespace", drd.Namespace)
			emitEvent.EmitEventGeneric(drd, "DruidOperatorPvcReSizeFail", e.Error(), e)
			return e
		}

		// If desired size > current STS VCT size, delete STS with cascade=orphan.
		// The operator on next reconcile shall create the STS with latest changes.
		if desiredVsCurrent > 0 && !stsDeleted {
			msg := fmt.Sprintf("Detected Change in VolumeClaimTemplate Sizes for StatefulSet [%s] in Namespace [%s], VCT [%s]: desired [%s], current [%s], deleting STS with cascade=orphan",
				statefulSet.Name, statefulSet.Namespace, desiredVCT.Name, desiredSize.String(), currentSize.String())
			logger.Info(msg)
			emitEvent.EmitEventGeneric(drd, "DruidOperatorPvcReSizeDetected", msg, nil)

			if err := writers.Delete(ctx, sdk, drd, stsObj, emitEvent, client.PropagationPolicy(metav1.DeletePropagationOrphan)); err != nil {
				return err
			}
			msg = fmt.Sprintf("[StatefulSet:%s] successfully deleted with cascade=orphan", statefulSet.Name)
			logger.Info(msg, "name", drd.Name, "namespace", drd.Namespace)
			emitEvent.EmitEventGeneric(drd, "DruidOperatorStsOrphaned", msg, nil)
			stsDeleted = true
		}

		// Expand only PVCs belonging to this VCT (never shrink)
		for _, pvc := range pvcsByVCT[desiredVCT.Name] {
			pvcSize := pvc.Spec.Resources.Requests[v1.ResourceStorage]
			if desiredSize.Cmp(pvcSize) > 0 {
				patch := client.MergeFrom(pvc.DeepCopy())
				pvc.Spec.Resources.Requests[v1.ResourceStorage] = desiredSize
				if err := writers.Patch(ctx, sdk, drd, pvc, false, patch, emitEvent); err != nil {
					return err
				}
				msg := fmt.Sprintf("[PVC:%s] successfully Patched with [Size:%s]", pvc.Name, desiredSize.String())
				logger.Info(msg, "name", drd.Name, "namespace", drd.Namespace)
			}
		}
	}

	return nil
}
