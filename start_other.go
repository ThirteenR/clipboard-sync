//go:build !windows

package main

import (
	"os/exec"
)

func setDetachAttr(cmd *exec.Cmd) {
	// 非Windows平台不需要特殊处理
}
