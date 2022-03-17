////////////////////////////////////////////////////////////////////////////////
// Utility to Determine Host Setup
////////////////////////////////////////////////////////////////////////////////
package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/shirou/gopsutil/host"
)

const divider = "______________________________________________________________________\n"

var completedActions struct {
	sync.Mutex
	num int
}

func actionCompleted() {
	completedActions.Lock()
	completedActions.num++
	completedActions.Unlock()
}

func numActionsCompleted() int {
	completedActions.Lock()
	num := completedActions.num
	completedActions.Unlock()
	return num
}

////////////////////////////////////////////////////////////////////////////////
// getOutboundIP takes an FQDN/IP and returns a string of the preferred
// outbound IP of this host to a given site
////////////////////////////////////////////////////////////////////////////////
func getOutboundIP(site string) string {
	conn, err := net.Dial("udp", site)
	defer conn.Close()

	if err != nil {
		// log.Fatal(err)
		return ""
	}

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	actionCompleted()

	return (localAddr.IP).String()
}

////////////////////////////////////////////////////////////////////////////////
// getPublicIP takes a public site/URL and returns a string containing the
// public IP address that this host appears to source from
////////////////////////////////////////////////////////////////////////////////
func getPublicIP(site string) string {
	// GET the info
	resp, err := http.Get(site)
	if err != nil {
		log.Fatalln(err)
		return "No IP (error)"
	}
	//We Read the response body on the line below.
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
		return "No IP (error)"
	}
	actionCompleted()

	return strings.TrimSuffix(string(body), "\n")
}

////////////////////////////////////////////////////////////////////////////////
// Main program loop
////////////////////////////////////////////////////////////////////////////////
func main() {
	// Store output of all information collected in single string
	var output strings.Builder
	// outputBound := binding.NewString()

	////////////////////////////////////////////////////////////////////////////
	// Store list of things to be collected so user can confirm it is ok
	////////////////////////////////////////////////////////////////////////////
	var collectItems strings.Builder

	// Add outbound IPs to check against
	for _, ip := range outboundIPs {
		collectItems.WriteString("Check outbound path to " + ip + "\n")
	}
	// Add public sites to check source IP against
	for _, site := range publicsites {
		collectItems.WriteString("Check source IP as seen from " + site + "\n")
	}
	// Add commands
	for _, command := range commands[runtime.GOOS] {
		collectItems.WriteString("Run command: " + command + "\n")
	}

	// Determine how many outbound IPs, public sites, and commands need to run
	// before the data collection is complete.  We will use this total to
	// calibrate the progress bar
	totalActions := len(outboundIPs) + len(publicsites) + len(commands[runtime.GOOS])

	// Record timestamp when run
	now := time.Now()
	output.WriteString("Diagnostics run on " + now.Format(time.RFC1123) + "\n")

	output.WriteString(divider)

	////////////////////////////////////////////////////////////////////////////
	// Get runtime OS/ARCH
	////////////////////////////////////////////////////////////////////////////
	osname := map[string]string{
		"darwin":  "Apple macOS",
		"linux":   "Linux",
		"windows": "Microsoft Windows",
	}
	chipset := map[string]string{
		"amd64": "AMD/Intel x64",
		"arm64": "Apple Silicon",
	}

	hostname, err := os.Hostname()
	if err != nil {
		hostname = ""
	}
	osplatform := fmt.Sprintf("%v (%v)", osname[runtime.GOOS], runtime.GOOS)
	osarch := fmt.Sprintf("%v (%v)", chipset[runtime.GOARCH], runtime.GOARCH)

	// Only thing we really want here is the OS version
	// platform, family, pver, err := host.PlatformInformation()
	_, _, osver, err := host.PlatformInformation()
	if err != nil {
		osver = ""
	}

	output.WriteString(fmt.Sprintf("HOSTNAME:          %v\n", hostname))
	output.WriteString(fmt.Sprintf("OPERATING SYSTEM:  %v\n", osplatform))
	output.WriteString(fmt.Sprintf("OS VERSION:        %v\n", osver))
	output.WriteString(fmt.Sprintf("ARCHITECTURE:      %v\n\n", osarch))

	output.WriteString(divider)

	////////////////////////////////////////////////////////////////////////////
	// Get Interface IP info
	////////////////////////////////////////////////////////////////////////////
	var ips []net.IP
	ifaces, err := net.Interfaces()
	if err == nil {
		for _, i := range ifaces {
			addrs, err := i.Addrs()
			if err == nil {
				for _, addr := range addrs {
					var ip net.IP
					switch v := addr.(type) {
					case *net.IPNet:
						ip = v.IP
					case *net.IPAddr:
						ip = v.IP
					}
					// process IP address
					ips = append(ips, ip)
					// output.WriteString(ip.String() + "\n")
				}
			}
		}
	}
	// Sort interface IPs
	sort.Slice(ips, func(i, j int) bool {
		return bytes.Compare(ips[i], ips[j]) < 0
	})
	// Add IPs to output
	var interfaceIPs strings.Builder
	output.WriteString("HOST INTERFACE IP ADDRESSES (SORTED):\n\n")
	for _, ip := range ips {
		addIP := ip.String() + "\n"
		interfaceIPs.WriteString(addIP)
		output.WriteString(addIP)
	}

	output.WriteString("\n" + divider)

	////////////////////////////////////////////////////////////////////////////
	// Create new Fyne application
	////////////////////////////////////////////////////////////////////////////
	myapp := app.New()

	////////////////////////////////////////////////////////////////////////////
	// Build MAIN window
	////////////////////////////////////////////////////////////////////////////
	window := myapp.NewWindow(title)
	window.Resize(fyne.NewSize(800, 600))

	// Create Full Output tab info
	// with binding to text variable initially set to current 'output'
	// We have to do this earlier because we tie this to [copy content]
	// button callback function
	fulloutputBound := binding.NewString()
	fulloutputBound.Set(output.String())
	fulloutput := widget.NewEntryWithData(fulloutputBound)
	fulloutput.TextStyle = fyne.TextStyle{Monospace: true}
	fulloutput.Disabled()

	// Create/initialize [copy content] button
	// Button copies 'content' into clipboard
	copybutton := widget.NewButtonWithIcon(
		"copy content",
		theme.ContentCopyIcon(),
		func() {
			if content, err := fulloutputBound.Get(); err == nil {
				window.Clipboard().SetContent(content)
			}
		},
	)
	// Disable button initially
	copybutton.Disable()

	////////////////////////////////////////////////////////////////////////////
	// Create/initialize progress bar at 0%
	progressbar := widget.NewProgressBar()
	progressbar.SetValue(0.0)

	////////////////////////////////////////////////////////////////////////////
	// Create bottom border using 2 column grid to hold
	// progress bar on left and [copy content] button on right.
	bottomBorder := container.NewGridWithColumns(2,
		progressbar,
		container.NewHBox(
			layout.NewSpacer(),
			copybutton,
		),
	)

	////////////////////////////////////////////////////////////////////////////
	// CREATE TABS

	// Create OS tab info
	osInfo := widget.NewForm()
	osInfo.Append("HOSTNAME:", widget.NewLabel(hostname))
	osInfo.Append("OPERATING SYSTEM:", widget.NewLabel(osplatform))
	osInfo.Append("OS VERSION:", widget.NewLabel(osver))
	osInfo.Append("ARCHITECTURE:", widget.NewLabel(osarch))
	osNote := widget.NewRichTextFromMarkdown("**NOTE:**  Simply click the button below, then paste into email/chat/ticket/etc.")
	// osNote.Wrapping = fyne.TextWrapWord
	osTabInfo := container.NewVBox(
		osInfo,
		layout.NewSpacer(),
		container.NewHBox(
			layout.NewSpacer(),
			osNote,
		),
	)

	// Create Host Interfaces tab info
	hostTabInfo := widget.NewForm()
	hostTabInfo.Append("Host interface IPs (sorted)", widget.NewLabel(interfaceIPs.String()))

	// Create Routing tab info
	routingTabInfo := widget.NewForm()
	routingTabInfo.Append("Destination", widget.NewLabel("IP of Local Interface that packet left on"))

	// Create Public IP tab info
	publicIPTabInfo := widget.NewForm()
	publicIPTabInfo.Append("This site", widget.NewLabel("sees this host coming from IP"))

	// Create Command Output tab info
	// commandTabInfo := widget.NewForm()
	// commandTabInfo.Append("Command output stuff", widget.NewLabel(osplatform))
	commandTabBound := binding.NewString()
	commandTabBound.Set("")
	commandTabContent := widget.NewLabelWithData(commandTabBound)
	commandTabContent.TextStyle = fyne.TextStyle{Monospace: true}
	commandTabInfo := container.NewScroll(commandTabContent)
	// commandTabInfo.Disabled()

	// Create application tabs for main window
	tabs := container.NewAppTabs(
		container.NewTabItem("OS", osTabInfo),
		container.NewTabItem("Host Interfaces", hostTabInfo),
		container.NewTabItem("Routing", routingTabInfo),
		container.NewTabItem("Public IP", publicIPTabInfo),
		container.NewTabItem("Command Output", commandTabInfo),
		container.NewTabItem("Full Output", fulloutput),
	)

	content := container.NewBorder(nil, bottomBorder, nil, nil, tabs)

	////////////////////////////////////////////////////////////////////////////
	// Build Confirmation dialog box
	////////////////////////////////////////////////////////////////////////////
	confirmContinue := dialog.NewConfirm(
		"May we collect the following information?",
		collectItems.String(),
		func(b bool) {
			if !b {
				myapp.Quit()
			} else {
				////////////////////////////////////////////////////////////////
				// Begin loop to update progress bar
				// and activate [copy content] button once info gathered
				////////////////////////////////////////////////////////////////
				go func() {
					for {
						time.Sleep(time.Millisecond * 250)
						progressbar.SetValue(float64(numActionsCompleted()) / float64(totalActions))
						if numActionsCompleted() == totalActions {
							// All actions completed, so we can stop this goroutine
							return
						}
					}
				}()

				////////////////////////////////////////////////////////////////
				// Fire off each action as a goroutine
				////////////////////////////////////////////////////////////////
				wg := sync.WaitGroup{}

				// Fire off tests to preferred IPs
				var preferredIPs = make([]string, len(outboundIPs))
				for index, ip := range outboundIPs {
					wg.Add(1)
					go func(i int, ip string) {
						defer wg.Done()
						result := getOutboundIP(ip + ":80")
						preferredIPs[i] = result
					}(index, ip)
				}

				// Fire off tests to find host's public IP
				var publicIPs = make([]string, len(publicsites))
				for index, site := range publicsites {
					wg.Add(1)
					go func(i int, url string) {
						defer wg.Done()
						result := getPublicIP(url)
						publicIPs[i] = result
					}(index, site)
				}

				// Fire off CLI commands
				var commandResults = make([]string, len(commands[runtime.GOOS]))
				for index, command := range commands[runtime.GOOS] {
					wg.Add(1)
					go func(i int, cmd string) {
						defer wg.Done()
						args := strings.Fields(cmd)
						out, err := exec.Command(args[0], args[1:]...).Output()
						result := ""
						if err == nil {
							result = string(out)
						}
						actionCompleted()
						commandResults[i] = result
					}(index, command)
				}

				wg.Wait()

				////////////////////////////////////////////////////////////////
				// Once all goroutines return, build Full Output tab content
				// and flesh out other tab data
				////////////////////////////////////////////////////////////////

				// Get preferred outbound IPs to various sites (helps show
				// route taken)
				output.WriteString("PREFERRED OUTBOUND IP (I.E., LOCAL INTERFACE)\n\n")
				for index, ip := range outboundIPs {
					routingTabInfo.Append("To "+ip, widget.NewLabel(preferredIPs[index]))
					// routingTabInfo.Append("To "+ip, widget.NewLabel(preferredIPs[index]))
					output.WriteString("To " + ip + ":  " + preferredIPs[index] + "\n")
				}
				output.WriteString("\n" + divider)

				// Get public IPs that host appears to source from when
				// visiting certain sites
				output.WriteString("PUBLIC SITES:\n\n")
				for index, site := range publicsites {
					publicIPTabInfo.Append(site, widget.NewLabel(publicIPs[index]))
					output.WriteString(site + " sees this host coming from " + publicIPs[index] + "\n")
				}

				// Get results of CLI commands run
				var commandOutput strings.Builder
				commandOutput.WriteString(divider)
				for index, command := range commands[runtime.GOOS] {
					commandOutput.WriteString("OUTPUT FROM RUNNING '" + command + "':\n\n")
					commandOutput.WriteString(strings.TrimSuffix(commandResults[index], "\n") + "\n")
					commandOutput.WriteString("\n" + divider)
				}
				commandTabBound.Set(commandOutput.String())
				fulloutputBound.Set(output.String() + commandOutput.String())

				// At this point, all the data is collected, so enable the button
				copybutton.Enable()
			}
		},
		window,
	)
	window.SetContent(content)
	window.Show()

	confirmContinue.Show()

	// Unleash the hounds!
	myapp.Run()
}
