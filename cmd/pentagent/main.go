// Command pentagent 是 Snag 的命令行客户端（§17 CLI、§39 CLI 对等）。
//
// CLI **只调本地 API、不碰库**（§17/§39.1）。用法：
//
//	pentagent run create [--name N]
//	pentagent run start   --id R
//	pentagent run pause   --id R [--mode now|after_current_action]
//	pentagent run resume  --id R
//	pentagent run stop    --id R
//	pentagent run status  --id R
//	pentagent run watch   --id R
//
// 全局开关：--json 稳定机读输出；--api 覆盖 API 地址（默认取 env PENTAGENT_API 或
// http://127.0.0.1:8787）。--yes 跳过本地确认，但**不**代表批准未来危险动作（§17）。
//
// 【有据偏离文档】dev-plan 建议 Cobra，但本环境网络权限关闭、无法拉取 github.com/spf13/cobra；
// 且本 CLI 命令面简单，标准库 flag + 子命令分发足矣、零外部依赖。保留 run <action> 命令形，
// 后续若需 Cobra 的补全/帮助体系可平滑替换。
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func newFlags(name string) *flag.FlagSet {
	return flag.NewFlagSet(name, flag.ContinueOnError)
}

// parseFlags 解析子命令 flags；若带 --api 则覆盖 base。解析失败返回 (2,false)。
func parseFlags(fs *flag.FlagSet, args []string, errw io.Writer, base *string) (int, bool) {
	fs.SetOutput(errw)
	if err := fs.Parse(args); err != nil {
		return 2, false
	}
	if f := fs.Lookup("api"); f != nil {
		if v := f.Value.String(); v != "" {
			*base = v
		}
	}
	return 0, true
}

func apiBase() string {
	if v := os.Getenv("PENTAGENT_API"); v != "" {
		return v
	}
	return "http://127.0.0.1:8787"
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 || args[0] != "run" {
		fmt.Fprintln(stderr, "用法: pentagent run <create|start|pause|resume|stop|status|watch> [flags]")
		return 2
	}
	if len(args) < 2 {
		fmt.Fprintln(stderr, "缺少动作: create|start|pause|resume|stop|status|watch")
		return 2
	}
	action, rest := args[1], args[2:]

	c := &cli{out: stdout, errw: stderr, base: apiBase()}
	switch action {
	case "create":
		return c.create(rest)
	case "start", "resume", "stop":
		return c.simple(action, rest)
	case "pause":
		return c.pause(rest)
	case "status":
		return c.status(rest)
	case "watch":
		return c.watch(rest)
	default:
		fmt.Fprintf(stderr, "未知动作 %q\n", action)
		return 2
	}
}

type cli struct {
	out  io.Writer
	errw io.Writer
	base string
}

type runResp struct {
	ID    string `json:"id"`
	State string `json:"state"`
	Mode  string `json:"mode,omitempty"`
}

// call 发一个请求，解析 JSON 响应体；非 2xx 返回 (body, status, nil) 让调用方据 status 处理。
func (c *cli) call(method, path string, body any) ([]byte, int, error) {
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.base+path, rdr)
	if err != nil {
		return nil, 0, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return data, resp.StatusCode, nil
}

// emit 据 --json 决定输出原始 JSON 还是人读一行。
func (c *cli) emit(jsonOut bool, data []byte, human string) {
	if jsonOut {
		c.out.Write(bytes.TrimSpace(data))
		c.out.Write([]byte("\n"))
		return
	}
	fmt.Fprintln(c.out, human)
}

// fail 打印错误（带 API 返回体）并返回退出码。
func (c *cli) fail(status int, data []byte) int {
	msg := strings.TrimSpace(string(data))
	fmt.Fprintf(c.errw, "错误 (HTTP %d): %s\n", status, msg)
	return 1
}

func (c *cli) create(args []string) int {
	fs := newFlags("create")
	name := fs.String("name", "", "Run 名称")
	jsonOut := fs.Bool("json", false, "JSON 输出")
	_ = fs.String("api", "", "API 地址（覆盖 env）")
	if code, ok := parseFlags(fs, args, c.errw, &c.base); !ok {
		return code
	}
	data, st, err := c.call("POST", "/v1/runs", map[string]string{"name": *name})
	if err != nil {
		fmt.Fprintf(c.errw, "请求失败: %v\n", err)
		return 1
	}
	if st != http.StatusCreated {
		return c.fail(st, data)
	}
	var rr runResp
	_ = json.Unmarshal(data, &rr)
	c.emit(*jsonOut, data, fmt.Sprintf("已创建 Run %s（%s）", rr.ID, rr.State))
	return 0
}

func (c *cli) simple(action string, args []string) int {
	fs := newFlags(action)
	id := fs.String("id", "", "Run id")
	jsonOut := fs.Bool("json", false, "JSON 输出")
	_ = fs.Bool("yes", false, "跳过本地确认（不代表批准未来危险动作）")
	_ = fs.String("api", "", "API 地址（覆盖 env）")
	if code, ok := parseFlags(fs, args, c.errw, &c.base); !ok {
		return code
	}
	if *id == "" {
		fmt.Fprintln(c.errw, "缺少 --id")
		return 2
	}
	data, st, err := c.call("POST", "/v1/runs/"+*id+"/"+action, nil)
	if err != nil {
		fmt.Fprintf(c.errw, "请求失败: %v\n", err)
		return 1
	}
	if st != http.StatusOK {
		return c.fail(st, data)
	}
	var rr runResp
	_ = json.Unmarshal(data, &rr)
	c.emit(*jsonOut, data, fmt.Sprintf("Run %s -> %s", rr.ID, rr.State))
	return 0
}

func (c *cli) pause(args []string) int {
	fs := newFlags("pause")
	id := fs.String("id", "", "Run id")
	mode := fs.String("mode", "after_current_action", "暂停档位: now|after_current_action")
	jsonOut := fs.Bool("json", false, "JSON 输出")
	_ = fs.String("api", "", "API 地址（覆盖 env）")
	if code, ok := parseFlags(fs, args, c.errw, &c.base); !ok {
		return code
	}
	if *id == "" {
		fmt.Fprintln(c.errw, "缺少 --id")
		return 2
	}
	data, st, err := c.call("POST", "/v1/runs/"+*id+"/pause?mode="+*mode, nil)
	if err != nil {
		fmt.Fprintf(c.errw, "请求失败: %v\n", err)
		return 1
	}
	if st != http.StatusOK {
		return c.fail(st, data)
	}
	var rr runResp
	_ = json.Unmarshal(data, &rr)
	c.emit(*jsonOut, data, fmt.Sprintf("Run %s -> %s（mode=%s）", rr.ID, rr.State, rr.Mode))
	return 0
}

func (c *cli) status(args []string) int {
	fs := newFlags("status")
	id := fs.String("id", "", "Run id")
	jsonOut := fs.Bool("json", false, "JSON 输出")
	_ = fs.String("api", "", "API 地址（覆盖 env）")
	if code, ok := parseFlags(fs, args, c.errw, &c.base); !ok {
		return code
	}
	if *id == "" {
		fmt.Fprintln(c.errw, "缺少 --id")
		return 2
	}
	data, st, err := c.call("GET", "/v1/runs/"+*id+"/status", nil)
	if err != nil {
		fmt.Fprintf(c.errw, "请求失败: %v\n", err)
		return 1
	}
	if st != http.StatusOK {
		return c.fail(st, data)
	}
	var rr runResp
	_ = json.Unmarshal(data, &rr)
	c.emit(*jsonOut, data, fmt.Sprintf("Run %s: %s", rr.ID, rr.State))
	return 0
}

// watch 轮询 status（SSE 流 Day 40 接）；非 TTY 下不交互，打印状态变化直到终态或轮询上限。
func (c *cli) watch(args []string) int {
	fs := newFlags("watch")
	id := fs.String("id", "", "Run id")
	_ = fs.Bool("json", false, "JSON 输出")
	_ = fs.String("api", "", "API 地址（覆盖 env）")
	if code, ok := parseFlags(fs, args, c.errw, &c.base); !ok {
		return code
	}
	if *id == "" {
		fmt.Fprintln(c.errw, "缺少 --id")
		return 2
	}
	terminal := map[string]bool{"COMPLETED": true, "PARTIAL": true, "STOPPED": true, "FAILED": true}
	last := ""
	for i := 0; i < 30; i++ {
		data, st, err := c.call("GET", "/v1/runs/"+*id+"/status", nil)
		if err != nil || st != http.StatusOK {
			return c.fail(st, data)
		}
		var rr runResp
		_ = json.Unmarshal(data, &rr)
		if rr.State != last {
			fmt.Fprintf(c.out, "Run %s: %s\n", rr.ID, rr.State)
			last = rr.State
		}
		if terminal[rr.State] {
			return 0
		}
		time.Sleep(200 * time.Millisecond)
	}
	return 0
}
