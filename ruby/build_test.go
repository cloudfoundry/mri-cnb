package ruby_test

import (
	"bytes"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cloudfoundry/packit"
	"github.com/cloudfoundry/packit/postal"
	"github.com/cloudfoundry/ruby-mri-cnb/ruby"
	"github.com/cloudfoundry/ruby-mri-cnb/ruby/fakes"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testBuild(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		layersDir         string
		cnbDir            string
		entryResolver     *fakes.EntryResolver
		dependencyManager *fakes.DependencyManager
		clock             ruby.Clock
		timeStamp         time.Time
		planRefinery      *fakes.BuildPlanRefinery
		buffer            *bytes.Buffer

		build packit.BuildFunc
	)

	it.Before(func() {
		var err error
		layersDir, err = ioutil.TempDir("", "layers")
		Expect(err).NotTo(HaveOccurred())

		cnbDir, err = ioutil.TempDir("", "cnb")
		Expect(err).NotTo(HaveOccurred())

		err = ioutil.WriteFile(filepath.Join(cnbDir, "buildpack.toml"), []byte(`api = "0.2"
[buildpack]
  id = "org.some-org.some-buildpack"
  name = "Some Buildpack"
  version = "some-version"

[metadata]
  [metadata.default-versions]
    ruby = "2.5.x"

  [[metadata.dependencies]]
    id = "some-dep"
    name = "Some Dep"
    sha256 = "some-sha"
    stacks = ["some-stack"]
    uri = "some-uri"
    version = "some-dep-version"
`), 0644)
		Expect(err).NotTo(HaveOccurred())

		entryResolver = &fakes.EntryResolver{}
		entryResolver.ResolveCall.Returns.BuildpackPlanEntry = packit.BuildpackPlanEntry{
			Name:    "ruby",
			Version: "2.5.x",
			Metadata: map[string]interface{}{
				"version-source": "buildpack.yml",
				"launch":         true,
			},
		}

		dependencyManager = &fakes.DependencyManager{}
		dependencyManager.ResolveCall.Returns.Dependency = postal.Dependency{Name: "Ruby MRI"}

		planRefinery = &fakes.BuildPlanRefinery{}

		timeStamp = time.Now()
		clock = ruby.NewClock(func() time.Time {
			return timeStamp
		})

		planRefinery.BillOfMaterialCall.Returns.BuildpackPlan = packit.BuildpackPlan{
			Entries: []packit.BuildpackPlanEntry{
				{
					Name:    "ruby",
					Version: "2.5.x",
					Metadata: map[string]interface{}{
						"version-source": "buildpack.yml",
						"launch":         true,
					},
				},
			},
		}

		buffer = bytes.NewBuffer(nil)
		logEmitter := ruby.NewLogEmitter(buffer)

		build = ruby.Build(entryResolver, dependencyManager, planRefinery, logEmitter, clock)
	})

	it.After(func() {
		Expect(os.RemoveAll(layersDir)).To(Succeed())
		Expect(os.RemoveAll(cnbDir)).To(Succeed())
	})

	it("returns a result that installs ruby", func() {
		result, err := build(packit.BuildContext{
			CNBPath: cnbDir,
			Stack:   "some-stack",
			BuildpackInfo: packit.BuildpackInfo{
				Name:    "Some Buildpack",
				Version: "some-version",
			},
			Plan: packit.BuildpackPlan{
				Entries: []packit.BuildpackPlanEntry{
					{
						Name:    "ruby",
						Version: "2.5.x",
						Metadata: map[string]interface{}{
							"version-source": "buildpack.yml",
							"launch":         true,
						},
					},
				},
			},
			Layers: packit.Layers{Path: layersDir},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(packit.BuildResult{
			Plan: packit.BuildpackPlan{
				Entries: []packit.BuildpackPlanEntry{
					{
						Name:    "ruby",
						Version: "2.5.x",
						Metadata: map[string]interface{}{
							"version-source": "buildpack.yml",
							"launch":         true,
						},
					},
				},
			},
			Layers: []packit.Layer{
				{
					Name:      "ruby",
					Path:      filepath.Join(layersDir, "ruby"),
					SharedEnv: packit.Environment{},
					BuildEnv:  packit.Environment{},
					LaunchEnv: packit.Environment{},
					Build:     false,
					Launch:    true,
					Cache:     false,
					Metadata: map[string]interface{}{
						ruby.DepKey: "",
						"built_at":  timeStamp.Format(time.RFC3339Nano),
					},
				},
			},
		}))

		Expect(filepath.Join(layersDir, "ruby")).To(BeADirectory())

		Expect(entryResolver.ResolveCall.Receives.BuildpackPlanEntrySlice).To(Equal([]packit.BuildpackPlanEntry{
			{
				Name:    "ruby",
				Version: "2.5.x",
				Metadata: map[string]interface{}{
					"version-source": "buildpack.yml",
					"launch":         true,
				},
			},
		}))

		Expect(dependencyManager.ResolveCall.Receives.Path).To(Equal(filepath.Join(cnbDir, "buildpack.toml")))
		Expect(dependencyManager.ResolveCall.Receives.Id).To(Equal("ruby"))
		Expect(dependencyManager.ResolveCall.Receives.Version).To(Equal("2.5.x"))
		Expect(dependencyManager.ResolveCall.Receives.Stack).To(Equal("some-stack"))

		Expect(planRefinery.BillOfMaterialCall.CallCount).To(Equal(1))
		Expect(planRefinery.BillOfMaterialCall.Receives.Dependency).To(Equal(postal.Dependency{Name: "Ruby MRI"}))

		Expect(dependencyManager.InstallCall.Receives.Dependency).To(Equal(postal.Dependency{Name: "Ruby MRI"}))
		Expect(dependencyManager.InstallCall.Receives.CnbPath).To(Equal(cnbDir))
		Expect(dependencyManager.InstallCall.Receives.LayerPath).To(Equal(filepath.Join(layersDir, "ruby")))

		Expect(buffer.String()).To(ContainSubstring("Some Buildpack some-version"))
		Expect(buffer.String()).To(ContainSubstring("Resolving Ruby MRI version"))
		Expect(buffer.String()).To(ContainSubstring("Selected Ruby MRI version (using buildpack.yml): "))
		Expect(buffer.String()).To(ContainSubstring("Executing build process"))
	})

	context("when the build plan entry includes the build flag", func() {
		var workingDir string

		it.Before(func() {
			var err error
			workingDir, err = ioutil.TempDir("", "working-dir")
			Expect(err).NotTo(HaveOccurred())

			entryResolver.ResolveCall.Returns.BuildpackPlanEntry = packit.BuildpackPlanEntry{
				Name: "ruby",

				Version: "2.5.x",
				Metadata: map[string]interface{}{
					"version-source": "buildpack.yml",
					"build":          true,
					"launch":         true,
				},
			}

			planRefinery.BillOfMaterialCall.Returns.BuildpackPlan = packit.BuildpackPlan{
				Entries: []packit.BuildpackPlanEntry{
					{
						Name:    "ruby",
						Version: "2.5.x",
						Metadata: map[string]interface{}{
							"version-source": "buildpack.yml",
							"build":          true,
							"launch":         true,
						},
					},
				},
			}
		})

		it.After(func() {
			Expect(os.RemoveAll(workingDir)).To(Succeed())
		})

		it("marks the ruby layer as cached", func() {
			result, err := build(packit.BuildContext{
				CNBPath:    cnbDir,
				Stack:      "some-stack",
				WorkingDir: workingDir,
				Plan: packit.BuildpackPlan{
					Entries: []packit.BuildpackPlanEntry{
						{
							Name:    "ruby",
							Version: "2.5.x",
							Metadata: map[string]interface{}{
								"version-source": "buildpack.yml",
								"build":          true,
								"launch":         true,
							},
						},
					},
				},
				Layers: packit.Layers{Path: layersDir},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(packit.BuildResult{
				Plan: packit.BuildpackPlan{
					Entries: []packit.BuildpackPlanEntry{
						{
							Name:    "ruby",
							Version: "2.5.x",
							Metadata: map[string]interface{}{
								"version-source": "buildpack.yml",
								"build":          true,
								"launch":         true,
							},
						},
					},
				},
				Layers: []packit.Layer{
					{
						Name:      "ruby",
						Path:      filepath.Join(layersDir, "ruby"),
						SharedEnv: packit.Environment{},
						BuildEnv:  packit.Environment{},
						LaunchEnv: packit.Environment{},
						Build:     true,
						Launch:    true,
						Cache:     true,
						Metadata: map[string]interface{}{
							ruby.DepKey: "",
							"built_at":  timeStamp.Format(time.RFC3339Nano),
						},
					},
				},
			}))
		})
	})

	context("when we refine the buildpack plan", func() {
		it.Before(func() {
			planRefinery.BillOfMaterialCall.Returns.BuildpackPlan = packit.BuildpackPlan{
				Entries: []packit.BuildpackPlanEntry{
					{
						Name:    "new-dep",
						Version: "some-version",
						Metadata: map[string]interface{}{
							"some-extra-field": "an-extra-value",
							"launch":           true,
						},
					},
				},
			}
		})
		it("refines the BuildpackPlan", func() {
			result, err := build(packit.BuildContext{
				CNBPath: cnbDir,
				Stack:   "some-stack",
				Plan: packit.BuildpackPlan{
					Entries: []packit.BuildpackPlanEntry{
						{
							Name:    "ruby",
							Version: "2.5.x",
							Metadata: map[string]interface{}{
								"version-source": "buildpack.yml",
								"launch":         true,
							},
						},
					},
				},
				Layers: packit.Layers{Path: layersDir},
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(result).To(Equal(packit.BuildResult{
				Plan: packit.BuildpackPlan{
					Entries: []packit.BuildpackPlanEntry{
						{
							Name:    "new-dep",
							Version: "some-version",
							Metadata: map[string]interface{}{
								"some-extra-field": "an-extra-value",
								"launch":           true,
							},
						},
					},
				},
				Layers: []packit.Layer{
					{
						Name:      "ruby",
						Path:      filepath.Join(layersDir, "ruby"),
						SharedEnv: packit.Environment{},
						BuildEnv:  packit.Environment{},
						LaunchEnv: packit.Environment{},
						Build:     false,
						Launch:    true,
						Cache:     false,
						Metadata: map[string]interface{}{
							ruby.DepKey: "",
							"built_at":  timeStamp.Format(time.RFC3339Nano),
						},
					},
				},
			}))
		})
	})

	context("when there is a dependency cache match", func() {
		it.Before(func() {
			err := ioutil.WriteFile(filepath.Join(layersDir, "ruby.toml"), []byte("[metadata]\ndependency-sha = \"some-sha\"\n"), 0644)
			Expect(err).NotTo(HaveOccurred())

			dependencyManager.ResolveCall.Returns.Dependency = postal.Dependency{
				Name:   "Ruby MRI",
				SHA256: "some-sha",
			}
		})

		it("exits build process early", func() {
			_, err := build(packit.BuildContext{
				CNBPath: cnbDir,
				Stack:   "some-stack",
				BuildpackInfo: packit.BuildpackInfo{
					Name:    "Some Buildpack",
					Version: "some-version",
				},
				Plan: packit.BuildpackPlan{
					Entries: []packit.BuildpackPlanEntry{
						{
							Name:    "ruby",
							Version: "2.5.x",
							Metadata: map[string]interface{}{
								"version-source": "buildpack.yml",
								"launch":         true,
							},
						},
					},
				},
				Layers: packit.Layers{Path: layersDir},
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(planRefinery.BillOfMaterialCall.CallCount).To(Equal(1))
			Expect(planRefinery.BillOfMaterialCall.Receives.Dependency).To(Equal(postal.Dependency{
				Name:   "Ruby MRI",
				SHA256: "some-sha",
			}))

			Expect(dependencyManager.InstallCall.CallCount).To(Equal(0))

			Expect(buffer.String()).To(ContainSubstring("Some Buildpack some-version"))
			Expect(buffer.String()).To(ContainSubstring("Resolving Ruby MRI version"))
			Expect(buffer.String()).To(ContainSubstring("Selected Ruby MRI version (using buildpack.yml): "))
			Expect(buffer.String()).To(ContainSubstring("Reusing cached layer"))
			Expect(buffer.String()).ToNot(ContainSubstring("Executing build process"))
		})
	})

	context("failure cases", func() {
		context("when a dependency cannot be resolved", func() {
			it.Before(func() {
				dependencyManager.ResolveCall.Returns.Error = errors.New("failed to resolve dependency")
			})

			it("returns an error", func() {
				_, err := build(packit.BuildContext{
					CNBPath: cnbDir,
					Plan: packit.BuildpackPlan{
						Entries: []packit.BuildpackPlanEntry{
							{
								Name:    "ruby",
								Version: "2.5.x",
								Metadata: map[string]interface{}{
									"version-source": "buildpack.yml",
								},
							},
						},
					},
					Layers: packit.Layers{Path: layersDir},
				})
				Expect(err).To(MatchError("failed to resolve dependency"))
			})
		})

		context("when a dependency cannot be installed", func() {
			it.Before(func() {
				dependencyManager.InstallCall.Returns.Error = errors.New("failed to install dependency")
			})

			it("returns an error", func() {
				_, err := build(packit.BuildContext{
					CNBPath: cnbDir,
					Plan: packit.BuildpackPlan{
						Entries: []packit.BuildpackPlanEntry{
							{
								Name:    "ruby",
								Version: "2.5.x",
								Metadata: map[string]interface{}{
									"version-source": "buildpack.yml",
								},
							},
						},
					},
					Layers: packit.Layers{Path: layersDir},
				})
				Expect(err).To(MatchError("failed to install dependency"))
			})
		})

		context("when the layers directory cannot be written to", func() {
			it.Before(func() {
				Expect(os.Chmod(layersDir, 0000)).To(Succeed())
			})

			it.After(func() {
				Expect(os.Chmod(layersDir, os.ModePerm)).To(Succeed())
			})

			it("returns an error", func() {
				_, err := build(packit.BuildContext{
					CNBPath: cnbDir,
					Plan: packit.BuildpackPlan{
						Entries: []packit.BuildpackPlanEntry{
							{
								Name:    "ruby",
								Version: "2.5.x",
								Metadata: map[string]interface{}{
									"version-source": "buildpack.yml",
								},
							},
						},
					},
					Layers: packit.Layers{Path: layersDir},
				})
				Expect(err).To(MatchError(ContainSubstring("permission denied")))
			})
		})

		context("when the layer directory cannot be removed", func() {
			var layerDir string
			it.Before(func() {
				layerDir = filepath.Join(layersDir, ruby.Ruby)
				Expect(os.MkdirAll(filepath.Join(layerDir, "baller"), os.ModePerm)).To(Succeed())
				Expect(os.Chmod(layerDir, 0000)).To(Succeed())
			})

			it.After(func() {
				Expect(os.Chmod(layerDir, os.ModePerm)).To(Succeed())
				Expect(os.RemoveAll(layerDir)).To(Succeed())
			})

			it("returns an error", func() {
				_, err := build(packit.BuildContext{
					CNBPath: cnbDir,
					Plan: packit.BuildpackPlan{
						Entries: []packit.BuildpackPlanEntry{
							{
								Name:    "ruby",
								Version: "2.5.x",
								Metadata: map[string]interface{}{
									"version-source": "buildpack.yml",
								},
							},
						},
					},
					Layers: packit.Layers{Path: layersDir},
				})
				Expect(err).To(MatchError(ContainSubstring("permission denied")))
			})
		})
	})
}