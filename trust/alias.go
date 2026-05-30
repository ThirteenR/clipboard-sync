package trust

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func (ts *TrustStore) PromptSetDeviceAlias(deviceUUID string) {
	reader := bufio.NewReader(os.Stdin)
	hostname, _ := os.Hostname()
	defaultAlias := hostname

	currentAlias := ts.GetDeviceAlias()
	if currentAlias != "" {
		fmt.Printf("当前设备别名: %s\n", currentAlias)
		fmt.Printf("UUID: %s\n", deviceUUID)
		fmt.Print("是否修改别名？(y/n): ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))
		if input != "y" && input != "yes" {
			return
		}
	}

	fmt.Printf("请输入设备别名 (最大20字符，当前默认: %s): ", defaultAlias)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" {
		input = defaultAlias
	}

	if err := ts.SetDeviceAlias(input); err != nil {
		fmt.Printf("设置别名失败: %v\n", err)
		return
	}

	fmt.Printf("设备别名已设置为: %s\n", input)
}

func (ts *TrustStore) RunAliasTUI(deviceUUID string) {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Println("\n=== Clipboard Sync 设备别名管理 ===")
		alias := ts.GetDeviceAlias()
		if alias == "" {
			alias = "(未设置)"
		}
		fmt.Printf("当前设备别名: %s\n", alias)
		fmt.Printf("UUID: %s\n", deviceUUID)

		fmt.Println("\n操作:")
		fmt.Println("  [1] 修改别名")
		fmt.Println("  [2] 查看其他设备别名")
		fmt.Println("  [q] 退出")
		fmt.Print("请选择: ")

		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))

		switch input {
		case "1":
			ts.PromptSetDeviceAlias(deviceUUID)
		case "2":
			ts.showPeerAliases()
		case "q":
			return
		default:
			fmt.Println("无效选择，请重新输入")
		}
	}
}

func (ts *TrustStore) showPeerAliases() {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	fmt.Println("\n=== 其他设备别名 ===")
	if ts.data.DeviceAliases == nil || len(ts.data.DeviceAliases) == 0 {
		fmt.Println("暂无其他设备别名记录")
		return
	}

	for uuid, alias := range ts.data.DeviceAliases {
		hostname := ""
		if dev, ok := ts.data.Devices[uuid]; ok {
			hostname = dev.Hostname
		}
		displayName := FormatDisplayName(alias, hostname, uuid)
		fmt.Printf("  %s\n", displayName)
	}
}

func (ts *TrustStore) SetAliasCommand(alias string, deviceUUID string) {
	if err := ts.SetDeviceAlias(alias); err != nil {
		fmt.Printf("设置别名失败: %v\n", err)
		return
	}
	fmt.Printf("设备别名已设置为: %s\n", alias)
}

func (ts *TrustStore) ShowAliasCommand() {
	alias := ts.GetDeviceAlias()
	if alias == "" {
		fmt.Println("设备别名未设置")
		return
	}
	fmt.Printf("设备别名: %s\n", alias)
}