package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

const defaultAddress = "127.0.0.1:19137"

func resolveAddress(flagValue string, flagSet bool) (string, error) {
	addr := flagValue
	if !flagSet {
		if port := strings.TrimSpace(os.Getenv("PORT")); port != "" {
			n, err := strconv.Atoi(port)
			if err != nil || n < 1024 || n > 65535 {
				return "", errors.New("PORT 必须是 1024 至 65535 的端口号")
			}
			addr = net.JoinHostPort("127.0.0.1", port)
		}
	}
	host, portText, err := net.SplitHostPort(addr)
	if err != nil {
		return "", fmt.Errorf("监听地址无效: %w", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return "", errors.New("监听端口必须在 1 至 65535 之间")
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return "", errors.New("监听地址必须使用回环 IP")
	}
	return net.JoinHostPort(host, strconv.Itoa(port)), nil
}
