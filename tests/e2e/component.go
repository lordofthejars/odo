package e2e

import (
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"io/ioutil"
	"strconv"
	"strings"
	"time"
)

// SourceTest checks the component-source-type and the source url in the annotation of the bc and dc
// appTestName is the name of the app
// sourceType is the type of the source of the component i.e git/binary/local
// source is the source of the component i.e gitURL or path to the directory or binary file
func SourceTest(appTestName string, sourceType string, source string) {
	// checking for source-type in dc
	getDc := runCmd("oc get dc wildfly-" + appTestName + " -o go-template='{{index .metadata.annotations \"app.kubernetes.io/component-source-type\"}}'")
	Expect(getDc).To(ContainSubstring(sourceType))

	// checking for source in dc
	getDc = runCmd("oc get dc wildfly-" + appTestName + " -o go-template='{{index .metadata.annotations \"app.kubernetes.io/url\"}}'")
	Expect(getDc).To(ContainSubstring(source))
}

func componentTests(componentCmdPrefix string) {
	const initContainerName = "copy-files-to-volume"
	const wildflyURI1 = "https://github.com/marekjelen/katacoda-odo-backend"
	const wildflyURI2 = "https://github.com/mik-dass/katacoda-odo-backend"
	const appRootVolumeName = "-testing-s2idata"

	t := strconv.FormatInt(time.Now().Unix(), 10)
	projName := generateTimeBasedName("odocmp")
	const appTestName = "testing"

	tmpDir, err := ioutil.TempDir("", "odoCmp")
	if err != nil {
		Fail(err.Error())
	}

	Context("odo component creation without application", func() {
		It("creating a component without an application should create one", func() {
			// new project == no app
			projectName := generateTimeBasedName("project")
			runCmd("odo project create " + projectName)
			Expect(runCmd("odo app list")).To(ContainSubstring("no applications"))

			const frontend = "frontend"
			// create a frontend component, an app should have been created
			runCmd(componentCmdPrefix + " create nodejs " + frontend)
			appName := getActiveElementFromCommandOutput("odo app list")
			Expect(appName).ToNot(BeEmpty())

			// check that we can get the component
			Expect(runCmd("odo component get")).To(ContainSubstring("The current component is: " + frontend))

			const backend = "backend"
			runCmd(componentCmdPrefix + " create python " + backend)
			Expect(runCmd("odo component get")).To(ContainSubstring("The current component is: " + backend))

			// switch back to frontend component
			Expect(runCmd("odo component set " + frontend)).To(ContainSubstring("Switched to component: " + frontend))

			// clean up
			runCmd("odo app delete " + appName + " -f")
			runCmd("odo project delete " + projectName + " -f")
			waitForDeleteCmd("odo project list", projectName)
		})
	})

	Context("odo component creation", func() {

		It("should create the project and application", func() {
			runCmd("odo project create " + projName)
			runCmd("odo app create " + appTestName)
		})

		It("should show an error when ref flag is provided with sources except git", func() {
			output := runFailCmd(fmt.Sprintf(componentCmdPrefix+" create nodejs cmp-git-%s --local test --ref test", t), 1)
			Expect(output).To(ContainSubstring("The --ref flag is only valid for --git flag"))
		})

		It("should create the component from the branch ref when provided", func() {
			runCmd(fmt.Sprintf(componentCmdPrefix+" create ruby ref-test-%s --git https://github.com/girishramnani/ruby-ex.git --ref develop", t))
			runCmd(fmt.Sprintf("odo url create ref-test-%s", t))

			routeURL := determineRouteURL() + "/health"
			waitForEqualCmd("curl -s "+routeURL+" | grep 'develop' | wc -l | tr -d '\n'", "1", 10)
		})

		It("should be able to create a component with git source", func() {
			runCmd(componentCmdPrefix + " create nodejs cmp-git --git https://github.com/openshift/nodejs-ex --min-memory 100Mi --max-memory 300Mi --min-cpu 0.1 --max-cpu 2")
			getMemoryLimit := runCmd("oc get dc cmp-git-" +
				appTestName +
				" -o go-template='{{range .spec.template.spec.containers}}{{.resources.limits.memory}}{{end}}'",
			)
			Expect(getMemoryLimit).To(ContainSubstring("300Mi"))
			getMemoryRequest := runCmd("oc get dc cmp-git-" +
				appTestName +
				" -o go-template='{{range .spec.template.spec.containers}}{{.resources.requests.memory}}{{end}}'",
			)
			Expect(getMemoryRequest).To(ContainSubstring("100Mi"))

			getCPULimit := runCmd("oc get dc cmp-git-" +
				appTestName +
				" -o go-template='{{range .spec.template.spec.containers}}{{.resources.limits.cpu}}{{end}}'",
			)
			Expect(getCPULimit).To(ContainSubstring("2"))
			getCPURequest := runCmd("oc get dc cmp-git-" +
				appTestName +
				" -o go-template='{{range .spec.template.spec.containers}}{{.resources.requests.cpu}}{{end}}'",
			)
			Expect(getCPURequest).To(ContainSubstring("100m"))
		})

		It("should list the component", func() {
			cmpList := runCmd(componentCmdPrefix + " list")
			Expect(cmpList).To(ContainSubstring("cmp-git"))
		})

		It("should be in component description", func() {
			cmpDesc := runCmd(componentCmdPrefix + " describe cmp-git")
			Expect(cmpDesc).To(ContainSubstring("Source: https://github.com/openshift/nodejs-ex"))
		})

		It("should be in application description", func() {
			appDesc := runCmd(componentCmdPrefix + " describe")
			Expect(appDesc).To(ContainSubstring("Source: https://github.com/openshift/nodejs-ex"))
		})

		It("should list the components in the catalog", func() {
			getProj := runCmd("odo catalog list components")
			Expect(getProj).To(ContainSubstring("wildfly"))
			Expect(getProj).To(ContainSubstring("ruby"))
			Expect(getProj).To(ContainSubstring("nodejs"))

			// check that the nodejs string does not contain the hidden versions
			lines := strings.Split(strings.Replace(getProj, "\r\n", "\n", -1), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "nodejs") {
					Expect(getProj).To(Not(ContainSubstring("0.10")))
				}
			}
		})
	})

	Context("updating the component", func() {
		It("should be able to create binary component", func() {
			runCmd("curl -L -o " + tmpDir + "/sample-binary-testing-1.war " +
				"https://gist.github.com/mik-dass/f95bd818ddba508ff76a386f8d984909/raw/e5bc575ac8b14ba2b23d66b5cb4873657e1a1489/sample.war")
			runCmd(componentCmdPrefix + " create wildfly wildfly --binary " + tmpDir + "/sample-binary-testing-1.war --memory 500Mi")

			// TODO: remove this once https://github.com/redhat-developer/odo/issues/943 is implemented
			time.Sleep(90 * time.Second)

			// Run push
			runCmd(componentCmdPrefix + " push -v 4")

			// Verify memory limits to be same as configured
			getMemoryLimit := runCmd("oc get dc wildfly-" +
				appTestName +
				" -o go-template='{{range .spec.template.spec.containers}}{{.resources.limits.memory}}{{end}}'",
			)
			Expect(getMemoryLimit).To(ContainSubstring("500Mi"))
			getMemoryRequest := runCmd("oc get dc wildfly-" +
				appTestName +
				" -o go-template='{{range .spec.template.spec.containers}}{{.resources.requests.memory}}{{end}}'",
			)
			Expect(getMemoryRequest).To(ContainSubstring("500Mi"))

			cmpList := runCmd(componentCmdPrefix + " list")
			Expect(cmpList).To(ContainSubstring("wildfly"))

			runCmd("oc get dc")
		})

		It("should update component from binary to binary", func() {
			runCmd("curl -L -o " + tmpDir + "/sample-binary-testing-2.war " +
				"'https://gist.github.com/mik-dass/f95bd818ddba508ff76a386f8d984909/raw/85354d9ee8583a9c1e64a331425eede235b07a9e/sample%2520(1).war'")

			waitForDCOfComponentToRolloutCompletely("wildfly")
			runCmd(componentCmdPrefix + " update wildfly --binary " + tmpDir + "/sample-binary-testing-2.war")

			// checking for init containers
			getDc := runCmd("oc get dc wildfly-" + appTestName + " -o go-template='" +
				"{{range .spec.template.spec.initContainers}}" +
				"{{.name}}{{end}}'")
			Expect(getDc).To(ContainSubstring(initContainerName))

			// checking for volumes
			getDc = runCmd("oc get dc wildfly-" + appTestName + " -o go-template='" +
				"{{range .spec.template.spec.volumes}}" +
				"{{.name}}{{end}}'")
			Expect(getDc).To(ContainSubstring("wildfly" + appRootVolumeName))

			// checking for volumes mounts
			getDc = runCmd("oc get dc wildfly-" + appTestName + " -o go-template='" +
				"{{range .spec.template.spec.containers}}{{range .volumeMounts}}{{.name}}" +
				"{{.name}}{{end}}{{end}}'")
			Expect(getDc).To(ContainSubstring("wildfly" + appRootVolumeName))

			SourceTest(appTestName, "binary", "file://"+tmpDir+"/sample-binary-testing-2.war")
		})

		It("should update component from binary to local", func() {
			runCmd("git clone " + wildflyURI1 + " " +
				tmpDir + "/katacoda-odo-backend-1")

			waitForDCOfComponentToRolloutCompletely("wildfly")
			runCmd(componentCmdPrefix + " update wildfly --local " + tmpDir + "/katacoda-odo-backend-1")

			// checking for init containers
			getDc := runCmd("oc get dc wildfly-" + appTestName + " -o go-template='" +
				"{{range .spec.template.spec.initContainers}}" +
				"{{.name}}{{end}}'")
			Expect(getDc).To(ContainSubstring(initContainerName))

			// checking for volumes
			getDc = runCmd("oc get dc wildfly-" + appTestName + " -o go-template='" +
				"{{range .spec.template.spec.volumes}}" +
				"{{.name}}{{end}}'")
			Expect(getDc).To(ContainSubstring("wildfly" + appRootVolumeName))

			// checking for volumes mounts
			getDc = runCmd("oc get dc wildfly-" + appTestName + " -o go-template='" +
				"{{range .spec.template.spec.containers}}{{range .volumeMounts}}{{.name}}" +
				"{{.name}}{{end}}{{end}}'")
			Expect(getDc).To(ContainSubstring("wildfly" + appRootVolumeName))

			// Verify memory limits to be same as configured
			getMemoryLimit := runCmd("oc get dc wildfly-" +
				appTestName +
				" -o go-template='{{range .spec.template.spec.containers}}{{.resources.limits.memory}}{{end}}'",
			)
			Expect(getMemoryLimit).To(ContainSubstring("500Mi"))
			getMemoryRequest := runCmd("oc get dc wildfly-" +
				appTestName +
				" -o go-template='{{range .spec.template.spec.containers}}{{.resources.requests.memory}}{{end}}'",
			)
			Expect(getMemoryRequest).To(ContainSubstring("500Mi"))

			SourceTest(appTestName, "local", "file://"+tmpDir+"/katacoda-odo-backend-1")
		})

		It("should update component from local to local", func() {
			runCmd("git clone " + wildflyURI2 + " " +
				tmpDir + "/katacoda-odo-backend-2")

			waitForDCOfComponentToRolloutCompletely("wildfly")
			runCmd(componentCmdPrefix + " update wildfly --local " + tmpDir + "/katacoda-odo-backend-2")

			// checking for init containers
			getDc := runCmd("oc get dc wildfly-" + appTestName + " -o go-template='" +
				"{{range .spec.template.spec.initContainers}}" +
				"{{.name}}{{end}}'")
			Expect(getDc).To(ContainSubstring(initContainerName))

			// checking for volumes
			getDc = runCmd("oc get dc wildfly-" + appTestName + " -o go-template='" +
				"{{range .spec.template.spec.volumes}}" +
				"{{.name}}{{end}}'")
			Expect(getDc).To(ContainSubstring("wildfly" + appRootVolumeName))

			// checking for volumes mounts
			getDc = runCmd("oc get dc wildfly-" + appTestName + " -o go-template='" +
				"{{range .spec.template.spec.containers}}{{range .volumeMounts}}{{.name}}" +
				"{{.name}}{{end}}{{end}}'")
			Expect(getDc).To(ContainSubstring("wildfly" + appRootVolumeName))

			SourceTest(appTestName, "local", "file://"+tmpDir+"/katacoda-odo-backend-2")
		})

		It("should update component from local to git", func() {
			waitForDCOfComponentToRolloutCompletely("wildfly")
			runCmd(componentCmdPrefix + " update wildfly --git " + wildflyURI1)

			// checking bc for updates
			getBc := runCmd("oc get bc wildfly-" + appTestName + " -o go-template={{.spec.source.git.uri}}")
			Expect(getBc).To(Equal(wildflyURI1))

			// checking for init containers
			getDc := runCmd("oc get dc wildfly-" + appTestName + " -o go-template='" +
				"{{range .spec.template.spec.initContainers}}" +
				"{{.name}}{{end}}'")
			Expect(getDc).NotTo(ContainSubstring(initContainerName))

			// checking for volumes
			getDc = runCmd("oc get dc wildfly-" + appTestName + " -o go-template='" +
				"{{range .spec.template.spec.volumes}}" +
				"{{.name}}{{end}}'")
			Expect(getDc).NotTo(ContainSubstring("wildfly" + appRootVolumeName))

			// checking for volumes mounts
			getDc = runCmd("oc get dc wildfly-" + appTestName + " -o go-template='" +
				"{{range .spec.template.spec.containers}}{{range .volumeMounts}}{{.name}}" +
				"{{.name}}{{end}}{{end}}'")
			Expect(getDc).NotTo(ContainSubstring("wildfly" + appRootVolumeName))

			SourceTest(appTestName, "git", wildflyURI1)
		})
		It("should update component from git to git", func() {
			waitForDCOfComponentToRolloutCompletely("wildfly")
			runCmd(componentCmdPrefix + " update wildfly --git " + wildflyURI2)

			// checking bc for updates
			getBc := runCmd("oc get bc wildfly-" + appTestName + " -o go-template={{.spec.source.git.uri}}")
			Expect(getBc).To(Equal(wildflyURI2))

			// checking for init containers
			getDc := runCmd("oc get dc wildfly-" + appTestName + " -o go-template='" +
				"{{range .spec.template.spec.initContainers}}" +
				"{{.name}}{{end}}'")
			Expect(getDc).NotTo(ContainSubstring(initContainerName))

			// checking for volumes
			getDc = runCmd("oc get dc wildfly-" + appTestName + " -o go-template='" +
				"{{range .spec.template.spec.volumes}}" +
				"{{.name}}{{end}}'")
			Expect(getDc).NotTo(ContainSubstring("wildfly" + appRootVolumeName))

			// checking for volumes mounts
			getDc = runCmd("oc get dc wildfly-" + appTestName + " -o go-template='" +
				"{{range .spec.template.spec.containers}}{{range .volumeMounts}}{{.name}}" +
				"{{.name}}{{end}}{{end}}'")
			Expect(getDc).NotTo(ContainSubstring("wildfly" + appRootVolumeName))

			SourceTest(appTestName, "git", wildflyURI2)
		})

		// This is expected to be removed at the time of fixing https://github.com/redhat-developer/odo/issues/1008
		It("should create a wildfly git component", func() {
			runCmd(componentCmdPrefix + " delete wildfly -f")
			runCmd(componentCmdPrefix + " create wildfly wildfly --git " + wildflyURI1)
		})

		It("should update component from git to local", func() {
			waitForDCOfComponentToRolloutCompletely("wildfly")
			runCmd(componentCmdPrefix + " update wildfly --local " + tmpDir + "/katacoda-odo-backend-1")

			// checking for init containers
			getDc := runCmd("oc get dc wildfly-" + appTestName + " -o go-template='" +
				"{{range .spec.template.spec.initContainers}}" +
				"{{.name}}{{end}}'")
			Expect(getDc).To(ContainSubstring(initContainerName))

			// checking for volumes
			getDc = runCmd("oc get dc wildfly-" + appTestName + " -o go-template='" +
				"{{range .spec.template.spec.volumes}}" +
				"{{.name}}{{end}}'")
			Expect(getDc).To(ContainSubstring("wildfly" + appRootVolumeName))

			// checking for volumes mounts
			getDc = runCmd("oc get dc wildfly-" + appTestName + " -o go-template='" +
				"{{range .spec.template.spec.containers}}{{range .volumeMounts}}{{.name}}" +
				"{{.name}}{{end}}{{end}}'")
			Expect(getDc).To(ContainSubstring("wildfly" + appRootVolumeName))

			SourceTest(appTestName, "local", "file://"+tmpDir+"/katacoda-odo-backend-1")
		})

		It("should update component from local to binary", func() {
			waitForDCOfComponentToRolloutCompletely("wildfly")
			runCmd(componentCmdPrefix + " update wildfly --binary " + tmpDir + "/sample-binary-testing-1.war")

			// checking for init containers
			getDc := runCmd("oc get dc wildfly-" + appTestName + " -o go-template='" +
				"{{range .spec.template.spec.initContainers}}" +
				"{{.name}}{{end}}'")
			Expect(getDc).To(ContainSubstring(initContainerName))

			// checking for volumes
			getDc = runCmd("oc get dc wildfly-" + appTestName + " -o go-template='" +
				"{{range .spec.template.spec.volumes}}" +
				"{{.name}}{{end}}'")
			Expect(getDc).To(ContainSubstring("wildfly" + appRootVolumeName))

			// checking for volumes mounts
			getDc = runCmd("oc get dc wildfly-" + appTestName + " -o go-template='" +
				"{{range .spec.template.spec.containers}}{{range .volumeMounts}}{{.name}}" +
				"{{.name}}{{end}}{{end}}'")
			Expect(getDc).To(ContainSubstring("wildfly" + appRootVolumeName))

			SourceTest(appTestName, "binary", "file://"+tmpDir+"/sample-binary-testing-1.war")
		})

		It("should create a wildfly git component", func() {
			runCmd(componentCmdPrefix + " delete wildfly -f")
			runCmd(componentCmdPrefix + " create wildfly wildfly --git " + wildflyURI1)
		})

		It("should update component from git to binary", func() {
			waitForDCOfComponentToRolloutCompletely("wildfly")
			runCmd(componentCmdPrefix + " update wildfly --binary " + tmpDir + "/sample-binary-testing-1.war")

			// checking for init containers
			getDc := runCmd("oc get dc wildfly-" + appTestName + " -o go-template='" +
				"{{range .spec.template.spec.initContainers}}" +
				"{{.name}}{{end}}'")
			Expect(getDc).To(ContainSubstring(initContainerName))

			// checking for volumes
			getDc = runCmd("oc get dc wildfly-" + appTestName + " -o go-template='" +
				"{{range .spec.template.spec.volumes}}" +
				"{{.name}}{{end}}'")
			Expect(getDc).To(ContainSubstring("wildfly" + appRootVolumeName))

			// checking for volumes mounts
			getDc = runCmd("oc get dc wildfly-" + appTestName + " -o go-template='" +
				"{{range .spec.template.spec.containers}}{{range .volumeMounts}}{{.name}}" +
				"{{.name}}{{end}}{{end}}'")
			Expect(getDc).To(ContainSubstring("wildfly" + appRootVolumeName))

			SourceTest(appTestName, "binary", "file://"+tmpDir+"/sample-binary-testing-1.war")
		})

		It("should update component from binary to git", func() {
			waitForDCOfComponentToRolloutCompletely("wildfly")
			runCmd(componentCmdPrefix + " update wildfly --git " + wildflyURI1)

			// checking bc for updates
			getBc := runCmd("oc get bc wildfly-" + appTestName + " -o go-template={{.spec.source.git.uri}}")
			Expect(getBc).To(Equal(wildflyURI1))

			// checking for init containers
			getDc := runCmd("oc get dc wildfly-" + appTestName + " -o go-template='" +
				"{{range .spec.template.spec.initContainers}}" +
				"{{.name}}{{end}}'")
			Expect(getDc).NotTo(ContainSubstring(initContainerName))

			// checking for volumes
			getDc = runCmd("oc get dc wildfly-" + appTestName + " -o go-template='" +
				"{{range .spec.template.spec.volumes}}" +
				"{{.name}}{{end}}'")
			Expect(getDc).NotTo(ContainSubstring("wildfly" + appRootVolumeName))

			// checking for volumes mounts
			getDc = runCmd("oc get dc wildfly-" + appTestName + " -o go-template='" +
				"{{range .spec.template.spec.containers}}{{range .volumeMounts}}{{.name}}" +
				"{{.name}}{{end}}{{end}}'")
			Expect(getDc).NotTo(ContainSubstring("wildfly" + appRootVolumeName))

			SourceTest(appTestName, "git", wildflyURI1)
		})

	})

	Context("cleaning up", func() {
		It("should delete the application", func() {
			runCmd("odo app delete " + appTestName + " -f")

			runCmd("odo project delete " + projName + " -f")
			waitForDeleteCmd("odo project list", projName)
		})
	})
}
