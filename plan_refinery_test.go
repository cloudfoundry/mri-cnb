package mri_test

import (
	"testing"

	"github.com/paketo-buildpacks/mri"
	"github.com/paketo-buildpacks/packit/postal"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testPlanRefinery(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		planRefinery mri.PlanRefinery
	)

	it.Before(func() {
		planRefinery = mri.NewPlanRefinery()
	})

	context("BillOfMaterial", func() {
		it("creates a buildpack plan entry from the given dependency", func() {
			refinedBuildPlan := planRefinery.BillOfMaterial(postal.Dependency{
				ID:      "some-id",
				Name:    "some-name",
				Stacks:  []string{"some-stack"},
				URI:     "some-uri",
				SHA256:  "some-sha",
				Version: "some-version",
			})
			Expect(refinedBuildPlan.Entries).To(HaveLen(1))
			Expect(refinedBuildPlan.Entries[0].Name).To(Equal("some-id"))
			Expect(refinedBuildPlan.Entries[0].Metadata).To(Equal(map[string]interface{}{
				"licenses": []string{},
				"name":     "some-name",
				"sha256":   "some-sha",
				"stacks":   []string{"some-stack"},
				"uri":      "some-uri",
				"version":  "some-version",
			},
			))
		})
	})
}
