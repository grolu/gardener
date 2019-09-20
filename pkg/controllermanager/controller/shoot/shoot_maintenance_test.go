// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package shoot_test

import (
	"time"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/shoot"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Shoot Maintenance", func() {
	now := time.Now()
	expirationDateInTheFuture := metav1.Time{Time: now.Add(time.Minute * 10)}
	expirationDateInThePast := metav1.Time{Time: now.AddDate(0, 0, -1)}
	trueVar := true
	falseVar := false

	Context("Shoot Maintenance", func() {
		Describe("ExpirationDateExpired", func() {
			It("should determine that expirationDate applies", func() {
				applies := ExpirationDateExpired(&expirationDateInThePast)
				Expect(applies).To(Equal(trueVar))
			})

			It("should determine that expirationDate not applies", func() {
				applies := ExpirationDateExpired(&expirationDateInTheFuture)
				Expect(applies).To(Equal(falseVar))
			})
		})

		Describe("ForceMachineImageUpdateRequired", func() {
			var (
				shootCurrentImage = &gardencorev1alpha1.ShootMachineImage{
					Name:    "CoreOs",
					Version: "1.0.0",
				}
			)

			It("should determine that forceUpdate is required", func() {
				imageCloudProvider := gardencorev1alpha1.MachineImage{
					Name: "CoreOs",
					Versions: []gardencorev1alpha1.ExpirableVersion{
						{
							Version: "1.0.1",
						},
						{
							Version:        "1.0.0",
							ExpirationDate: &expirationDateInThePast,
						},
					},
				}

				required := ForceMachineImageUpdateRequired(shootCurrentImage, imageCloudProvider)
				Expect(required).To(Equal(trueVar))
			})

			It("should determine that forceUpdate is not required", func() {
				imageCloudProvider := gardencorev1alpha1.MachineImage{
					Name: "CoreOs",
					Versions: []gardencorev1alpha1.ExpirableVersion{
						{
							Version: "1.0.1",
						},
						{
							Version:        "1.0.0",
							ExpirationDate: &expirationDateInTheFuture,
						},
					},
				}

				required := ForceMachineImageUpdateRequired(shootCurrentImage, imageCloudProvider)
				Expect(required).To(Equal(falseVar))
			})
		})
	})

	Describe("MaintainMachineImages", func() {
		var (
			shootCurrentImage    *gardencorev1alpha1.ShootMachineImage
			cloudProfile         *gardencorev1alpha1.CloudProfile
			shoot                *gardencorev1alpha1.Shoot
			machineCurrentImages []*gardencorev1alpha1.ShootMachineImage
		)

		BeforeEach(func() {
			shootCurrentImage = &gardencorev1alpha1.ShootMachineImage{
				Name:    "CoreOs",
				Version: "1.0.0",
			}

			machineCurrentImages = []*gardencorev1alpha1.ShootMachineImage{shootCurrentImage}

			cloudProfile = &gardencorev1alpha1.CloudProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: "profile",
				},
				Spec: gardencorev1alpha1.CloudProfileSpec{
					MachineImages: []gardencorev1alpha1.MachineImage{
						{
							Name: "CoreOs",
							Versions: []gardencorev1alpha1.ExpirableVersion{
								{
									Version: "1.0.0",
								},
								{
									Version:        "1.1.1",
									ExpirationDate: &expirationDateInTheFuture,
								},
							},
						},
					},
				},
			}

			shoot = &gardencorev1alpha1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name: "shoot",
				},
				Spec: gardencorev1alpha1.ShootSpec{
					Maintenance: &gardencorev1alpha1.Maintenance{
						AutoUpdate: &gardencorev1alpha1.MaintenanceAutoUpdate{
							MachineImageVersion: &trueVar,
						},
					},
				},
			}
		})

		It("should determine that the shoot worker machine images must be maintained - ForceUpdate", func() {
			shoot.Spec.Maintenance.AutoUpdate.MachineImageVersion = &falseVar
			cloudProfile.Spec.MachineImages[0].Versions[0].ExpirationDate = &expirationDateInThePast

			workerImages, err := MaintainMachineImages(shoot, cloudProfile, shootCurrentImage, machineCurrentImages)

			Expect(err).To(BeNil())
			Expect(len(workerImages)).NotTo(Equal(0))
			Expect(workerImages[0].Name).To(Equal(cloudProfile.Spec.MachineImages[0].Name))
			Expect(workerImages[0].Version).To(Equal(cloudProfile.Spec.MachineImages[0].Versions[1].Version))
		})

		It("should determine that the shoot worker machine images must be maintained - MaintenanceAutoUpdate set to true (nil is also is being defaulted to true in the apiserver)", func() {
			workerImages, err := MaintainMachineImages(shoot, cloudProfile, shootCurrentImage, machineCurrentImages)

			Expect(err).To(BeNil())
			Expect(len(workerImages)).NotTo(Equal(0))
			Expect(workerImages[0].Name).To(Equal(cloudProfile.Spec.MachineImages[0].Name))
			Expect(workerImages[0].Version).To(Equal(cloudProfile.Spec.MachineImages[0].Versions[1].Version))
		})

		It("should determine that the shoot worker machine images must NOT to be maintained - ForceUpdate not required & MaintenanceAutoUpdate set to false", func() {
			shoot.Spec.Maintenance.AutoUpdate.MachineImageVersion = &falseVar

			workerImages, err := MaintainMachineImages(shoot, cloudProfile, shootCurrentImage, machineCurrentImages)

			Expect(err).To(BeNil())
			Expect(len(workerImages)).To(Equal(0))
		})

		It("should determine that the shoot worker machine images must be maintained - cloud profile has no matching (machineImage.name & machineImage.version) machine image defined (the shoots image has been deleted from the cloudProfile) -> update to latest machineImage with same name", func() {
			cloudProfile.Spec.MachineImages = []gardencorev1alpha1.MachineImage{
				{
					Name: "CoreOs",
					Versions: []gardencorev1alpha1.ExpirableVersion{
						{
							Version:        "1.1.1",
							ExpirationDate: &expirationDateInTheFuture,
						},
					},
				},
			}

			workerImages, err := MaintainMachineImages(shoot, cloudProfile, shootCurrentImage, machineCurrentImages)

			Expect(err).To(BeNil())
			Expect(len(workerImages)).NotTo(Equal(0))
			Expect(workerImages[0].Name).To(Equal(cloudProfile.Spec.MachineImages[0].Name))
			Expect(workerImages[0].Version).To(Equal(cloudProfile.Spec.MachineImages[0].Versions[0].Version))
		})

		It("should return an error - cloud profile has no matching (machineImage.name) machine image defined", func() {
			cloudProfile.Spec.MachineImages = cloudProfile.Spec.MachineImages[1:]

			_, err := MaintainMachineImages(shoot, cloudProfile, shootCurrentImage, machineCurrentImages)

			Expect(err).NotTo(BeNil())
		})
	})

	Describe("Maintain Kubernetes Version", func() {
		var (
			cloudProfile       *gardencorev1alpha1.CloudProfile
			shoot              *gardencorev1alpha1.Shoot
			kubernetesSettings gardencorev1alpha1.KubernetesConstraints
		)

		BeforeEach(func() {
			kubernetesSettings = gardencorev1alpha1.KubernetesConstraints{
				Versions: []gardencorev1alpha1.ExpirableVersion{
					{
						Version: "1.0.2",
					},
					{
						Version: "1.0.1",
					},
					{
						Version:        "1.0.0",
						ExpirationDate: &expirationDateInTheFuture,
					},
				},
			}
			cloudProfile = &gardencorev1alpha1.CloudProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: "profile",
				},
				Spec: gardencorev1alpha1.CloudProfileSpec{
					Kubernetes: kubernetesSettings,
				},
			}

			shoot = &gardencorev1alpha1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name: "shoot",
				},
				Spec: gardencorev1alpha1.ShootSpec{
					Maintenance: &gardencorev1alpha1.Maintenance{
						AutoUpdate: &gardencorev1alpha1.MaintenanceAutoUpdate{
							KubernetesVersion: true,
						},
					},
					Kubernetes: gardencorev1alpha1.Kubernetes{Version: "1.0.0"},
				},
			}
		})
		It("should determine that the shoot kubernetes version must be maintained - ForceUpdate", func() {
			shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion = falseVar
			cloudProfile.Spec.Kubernetes.Versions[2].ExpirationDate = &expirationDateInThePast
			shoot.Spec.Kubernetes = gardencorev1alpha1.Kubernetes{Version: cloudProfile.Spec.Kubernetes.Versions[2].Version}

			version, err := MaintainKubernetesVersion(shoot, cloudProfile)

			Expect(err).To(BeNil())
			Expect(version).NotTo(BeNil())
			Expect(*version).To(Equal("1.0.2"))
		})

		It("should determine that the shoot kubernetes version must be maintained - MaintenanceAutoUpdate set to true", func() {
			shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion = trueVar
			cloudProfile.Spec.Kubernetes.Versions[2].ExpirationDate = &expirationDateInTheFuture
			shoot.Spec.Kubernetes = gardencorev1alpha1.Kubernetes{Version: cloudProfile.Spec.Kubernetes.Versions[2].Version}

			version, err := MaintainKubernetesVersion(shoot, cloudProfile)

			Expect(err).To(BeNil())
			Expect(version).NotTo(BeNil())
			Expect(*version).To(Equal("1.0.2"))
		})

		It("should determine that the kubernetes version must NOT to be maintained - ForceUpdate not required & MaintenanceAutoUpdate set to false", func() {
			shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion = falseVar
			cloudProfile.Spec.Kubernetes.Versions[2].ExpirationDate = &expirationDateInTheFuture
			shoot.Spec.Kubernetes = gardencorev1alpha1.Kubernetes{Version: cloudProfile.Spec.Kubernetes.Versions[2].Version}

			version, err := MaintainKubernetesVersion(shoot, cloudProfile)

			Expect(err).To(BeNil())
			Expect(version).To(BeNil())
		})

		It("should determine that the shootKubernetes version must be maintained - cloud profile has no matching kubernetes version defined (the shoots kubernetes version has been deleted from the cloudProfile) -> update to latest kubernetes patch version with same minor", func() {
			cloudProfile.Spec.Kubernetes.Versions = kubernetesSettings.Versions[:2]
			shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion = true
			shoot.Spec.Kubernetes = gardencorev1alpha1.Kubernetes{Version: "1.0.0"}

			version, err := MaintainKubernetesVersion(shoot, cloudProfile)

			Expect(err).To(BeNil())
			Expect(version).NotTo(BeNil())
			Expect(*version).To(Equal("1.0.2"))
		})

		It("should determine that the shootKubernetes version must NOT be maintained - cloud profile has no matching kubernetes version defined & autoUpdate == false", func() {
			cloudProfile.Spec.Kubernetes.Versions = kubernetesSettings.Versions[:2]
			shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion = false
			shoot.Spec.Kubernetes = gardencorev1alpha1.Kubernetes{Version: "1.0.0"}

			version, err := MaintainKubernetesVersion(shoot, cloudProfile)

			Expect(err).To(BeNil())
			Expect(version).To(BeNil())
		})
	})
})
