package cli

import (
	"fmt"
	"io"
	"strings"
	"time"

	"gsprobe/internal/model"
)

func PrintSummary(w io.Writer, r model.Report) {
	duration := r.EndedAt.Sub(r.StartedAt)
	if duration < 0 {
		duration = 0
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "====================================")
	fmt.Fprintln(w, " GSNode 检测完成")
	fmt.Fprintln(w, "====================================")
	fmt.Fprintf(w, "报告编号 : %s\n", strings.ToUpper(r.ID))
	fmt.Fprintf(w, "受检主机 : %s\n", r.Hostname)
	fmt.Fprintf(w, "运行平台 : %s\n", r.Platform)
	fmt.Fprintf(w, "客户端   : v%s (%s)\n", r.Version, r.Mode)
	fmt.Fprintf(w, "检测耗时 : %s\n", duration.Round(time.Second))
	fmt.Fprintf(w, "综合评分 : %s / 10000  %s\n", colorScore(r.Score), stars(r.Stars))
	fmt.Fprintln(w)
	fmt.Fprintln(w)

	printReportTables(w, r.Sections)

	fmt.Fprintln(w)
	fmt.Fprintln(w, "====================================")
}

func PrintUploadResult(w io.Writer, reportURL string, uploadErr error, localPath string) {
	if uploadErr != nil {
		fmt.Fprintln(w, "上传状态 : 失败")
		fmt.Fprintf(w, "原因     : %v\n", uploadErr)
		fmt.Fprintln(w)
		fmt.Fprintln(w, "终端上方已显示完整检测结果。")
		if localPath != "" {
			fmt.Fprintf(w, "本地报告 : %s\n", localPath)
		}
		fmt.Fprintln(w)
		return
	}
	if reportURL == "" {
		fmt.Fprintln(w, "上传状态 : 已跳过")
		fmt.Fprintln(w)
		return
	}
	fmt.Fprintln(w, "上传状态 : 成功")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "在线报告 :")
	link := hyperlink(reportURL, reportURL)
	if colorEnabled() {
		link = paint(colorAccent, link)
	}
	fmt.Fprintf(w, "  %s\n", link)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "在浏览器打开上述链接可查看完整证书式报告，可导出图片格式。")
	fmt.Fprintln(w)
}
