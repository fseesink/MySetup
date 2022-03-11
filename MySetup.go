package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const divider = "______________________________________________________________________\n"

////////////////////////////////////////////////////////////////////////////////
// GetOutboundIP returns a string of the preferred outbound IP of this host
// to a given site
////////////////////////////////////////////////////////////////////////////////
func GetOutboundIP(site string) string {
	conn, err := net.Dial("udp", site)
	defer conn.Close()

	if err != nil {
		// log.Fatal(err)
		return ""
	}

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return (localAddr.IP).String()
}

////////////////////////////////////////////////////////////////////////////////
// GetPublicIPs returns a string containing public site/URL(s) and the public
// IP address this host appears to source from
////////////////////////////////////////////////////////////////////////////////
func GetPublicIPs() string {
	var result strings.Builder
	for _, site := range publicsites {
		// GET the info
		resp, err := http.Get(site)
		if err != nil {
			log.Fatalln(err)
		}
		//We Read the response body on the line below.
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Fatalln(err)
		}
		//Convert the body to type string and add to result
		result.WriteString(site + " sees this host coming from " + strings.TrimSuffix(string(body), "\n") + "\n")
	}
	return result.String()
}

////////////////////////////////////////////////////////////////////////////////
// Main program loop
////////////////////////////////////////////////////////////////////////////////
func main() {
	var output strings.Builder

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

	appinfo := fmt.Sprintf("OPERATING SYSTEM: %v (%v)\nARCHITECTURE: %v (%v)\n", osname[runtime.GOOS], runtime.GOOS, chipset[runtime.GOARCH], runtime.GOARCH)
	output.WriteString(appinfo)

	output.WriteString(divider)

	////////////////////////////////////////////////////////////////////////////
	// Get preferred outbound IPs to various sites (helps show route taken)
	////////////////////////////////////////////////////////////////////////////
	for _, ip := range outboundIPs {
		output.WriteString("PREFERRED OUTBOUND IP (to " + ip + "):" + GetOutboundIP(ip+":80") + "\n")
	}

	output.WriteString(divider)

	////////////////////////////////////////////////////////////////////////////
	// Get public IP host appears to source from when visiting certain sites
	////////////////////////////////////////////////////////////////////////////
	output.WriteString("PUBLIC SITES:\n" + GetPublicIPs() + "\n")

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
	output.WriteString("HOST INTERFACE IP ADDRESSES (SORTED):\n")
	for _, ip := range ips {
		output.WriteString(ip.String() + "\n")
	}

	output.WriteString(divider)

	////////////////////////////////////////////////////////////////////////////
	// Get results of CLI commands to run
	////////////////////////////////////////////////////////////////////////////
	for _, command := range commands[runtime.GOOS] {
		args := strings.Fields(command)
		output.WriteString("OUTPUT FROM RUNNING '" + command + "':\n" + divider[len(divider)/2:])
		out, err := exec.Command(args[0], args[1:]...).Output()
		if err == nil {
			output.WriteString(string(out) + "\n")
		}
		output.WriteString(divider[len(divider)/2:])
	}

	output.WriteString(divider)

	myapp := app.New()
	window := myapp.NewWindow(title)
	window.Resize(fyne.NewSize(640, 480))

	txtBound := binding.NewString()
	labelWithData := widget.NewEntryWithData(txtBound)
	labelWithData.Disabled()

	bottomBox := container.NewHBox(
		layout.NewSpacer(),
		widget.NewButtonWithIcon("copy content", theme.ContentCopyIcon(), func() {
			if content, err := txtBound.Get(); err == nil {
				window.Clipboard().SetContent(content)
			}
		}),
	)

	content := container.NewBorder(nil, bottomBox, nil, nil, labelWithData)

	txtBound.Set(output.String())
	window.SetContent(content)

	window.ShowAndRun()
}
