/*
Copyright 2023 The KodeRover Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kube

// func EnsureUpdateZadigService(ctx context.Context, env *commonmodels.Product, svcName string, kclient client.Client, istioClient versionedclient.Interface) error {
// 	if !env.ShareEnv.Enable {
// 		return nil
// 	}

// 	// Note: A Service may not be queried immediately after it is created.
// 	var err error
// 	svc := &corev1.Service{}
// 	for i := 0; i < 3; i++ {
// 		err = kclient.Get(ctx, client.ObjectKey{
// 			Name:      svcName,
// 			Namespace: env.Namespace,
// 		}, svc)
// 		if err == nil {
// 			break
// 		}

// 		log.Warnf("Failed to query Service %s in ns %s: %s", svcName, env.Namespace, err)
// 		time.Sleep(1 * time.Second)
// 	}
// 	if err != nil {
// 		return fmt.Errorf("failed to query Service %s in ns %s: %s", svcName, env.Namespace, err)
// 	}

// 	return ensureUpdateZadigSerivce(ctx, env, svc, kclient, istioClient)
// }
