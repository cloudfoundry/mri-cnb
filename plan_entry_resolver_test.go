package mri_test

import (
	"bytes"
	"testing"

	"github.com/paketo-buildpacks/packit"
	"github.com/paketo-community/mri"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testPlanEntryResolver(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		buffer   *bytes.Buffer
		resolver mri.PlanEntryResolver
	)

	it.Before(func() {
		buffer = bytes.NewBuffer(nil)
		resolver = mri.NewPlanEntryResolver(mri.NewLogEmitter(buffer))
	})

	context("when a buildpack.yml entry is included", func() {
		it("resolves the best plan entry", func() {
			entry := resolver.Resolve([]packit.BuildpackPlanEntry{
				{
					Name: "mri",
					Metadata: map[string]interface{}{
						"version": "other-version",
					},
				},
				{
					Name: "mri",
					Metadata: map[string]interface{}{
						"version-source": "buildpack.yml",
						"version":        "buildpack-yml-version",
					},
				},
			})
			Expect(entry).To(Equal(packit.BuildpackPlanEntry{
				Name: "mri",
				Metadata: map[string]interface{}{
					"version-source": "buildpack.yml",
					"version":        "buildpack-yml-version",
				},
			}))

			Expect(buffer.String()).To(ContainSubstring("    Candidate version sources (in priority order):"))
			Expect(buffer.String()).To(ContainSubstring("      buildpack.yml -> \"buildpack-yml-version\""))
			Expect(buffer.String()).To(ContainSubstring("      <unknown>     -> \"other-version\""))
		})
	})

	context("when entry flags differ", func() {
		context("OR's them together on best plan entry", func() {
			it("has all flags", func() {
				entry := resolver.Resolve([]packit.BuildpackPlanEntry{
					{
						Name: "mri",
						Metadata: map[string]interface{}{
							"version-source": "buildpack.yml",
							"version":        "buildpack-yml-version",
						},
					},
					{
						Name: "mri",
						Metadata: map[string]interface{}{
							"build": true,
						},
					},
				})
				Expect(entry).To(Equal(packit.BuildpackPlanEntry{
					Name: "mri",
					Metadata: map[string]interface{}{
						"version-source": "buildpack.yml",
						"version":        "buildpack-yml-version",
						"build":          true,
					},
				}))
			})
		})
	})

	context("when an unknown source entry is included", func() {
		it("resolves the best plan entry", func() {
			entry := resolver.Resolve([]packit.BuildpackPlanEntry{
				{
					Name: "mri",
					Metadata: map[string]interface{}{
						"version": "other-version",
					},
				},
			})
			Expect(entry).To(Equal(packit.BuildpackPlanEntry{
				Name: "mri",
				Metadata: map[string]interface{}{
					"version": "other-version",
				},
			}))
		})
	})
}
