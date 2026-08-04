package probe

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

type provinceLatencyResult struct {
	Code     string  `json:"code"`
	Short    string  `json:"short"`
	Name     string  `json:"name"`
	Carrier  string  `json:"carrier"`
	Host     string  `json:"host"`
	Ms       float64 `json:"ms,omitempty"`
	Text     string  `json:"text"`
	Loss     float64 `json:"loss"`
	LossText string  `json:"lossText,omitempty"`
}

type chinaProvinceDef struct {
	Code  string
	Short string
	Name  string
}

var chinaProvinces = []chinaProvinceDef{
	{"BJ", "京", "北京"}, {"TJ", "津", "天津"}, {"HE", "冀", "河北"}, {"SX", "晋", "山西"},
	{"NM", "蒙", "内蒙古"}, {"LN", "辽", "辽宁"}, {"JL", "吉", "吉林"}, {"HL", "黑", "黑龙江"},
	{"SH", "沪", "上海"}, {"JS", "苏", "江苏"}, {"ZJ", "浙", "浙江"}, {"AH", "皖", "安徽"},
	{"FJ", "闽", "福建"}, {"JX", "赣", "江西"}, {"SD", "鲁", "山东"}, {"HA", "豫", "河南"},
	{"HB", "鄂", "湖北"}, {"HN", "湘", "湖南"}, {"GD", "粤", "广东"}, {"GX", "桂", "广西"},
	{"HI", "琼", "海南"}, {"CQ", "渝", "重庆"}, {"SC", "川", "四川"}, {"GZ", "贵", "贵州"},
	{"YN", "云", "云南"}, {"XZ", "藏", "西藏"}, {"SN", "陕", "陕西"}, {"GS", "甘", "甘肃"},
	{"QH", "青", "青海"}, {"NX", "宁", "宁夏"}, {"XJ", "新", "新疆"},
	{"HK", "港", "香港"}, {"MO", "澳", "澳门"}, {"TW", "台", "台湾"},
}

var chinaCarrierNames = []string{"电信", "联通", "移动"}

// chinaLatencyConcurrency 按可用内存限制并发 ping 数，避免低配容器 OOM。
func chinaLatencyConcurrency() int {
	const (
		defaultC = 20
		minC     = 2
		maxC     = 20
	)
	avail := memAvailableBytes()
	if avail == 0 {
		return defaultC
	}
	mb := avail / (1024 * 1024)
	var c int
	switch {
	case mb < 200:
		c = 5
	case mb < 400:
		c = 7
	case mb < 800:
		c = 10
	case mb < 1536:
		c = 14
	default:
		c = maxC
	}
	if c < minC {
		c = minC
	}
	if c > maxC {
		c = maxC
	}
	return c
}

// chinaLatencyProbeTimeout 按并发度估算总超时，低并发时自动延长。
// 每波耗时按「单点 ping 6s + 最多 2 次 failover」估算，避免低配容器提前取消。
func chinaLatencyProbeTimeout(concurrency, total int) time.Duration {
	if concurrency < 1 {
		concurrency = 1
	}
	if total < 1 {
		total = 1
	}
	waves := (total + concurrency - 1) / concurrency
	perWave := 10
	if concurrency <= 7 {
		perWave = 15
	}
	sec := waves*perWave + 80
	const minSec, maxSec = 120, 600
	if sec < minSec {
		sec = minSec
	}
	if sec > maxSec {
		sec = maxSec
	}
	return time.Duration(sec) * time.Second
}

func probeChinaProvinces(ctx context.Context, log Logger, targets *PingTargetDB) []provinceLatencyResult {
	total := len(chinaProvinces) * len(chinaCarrierNames)
	concurrency := chinaLatencyConcurrency()
	if avail := memAvailableBytes(); avail > 0 {
		log(fmt.Sprintf("ChinaLatency: 并发 %d（可用内存 %.0f MiB）", concurrency, float64(avail)/(1024*1024)))
	} else {
		log(fmt.Sprintf("ChinaLatency: 并发 %d", concurrency))
	}
	log(fmt.Sprintf("Route: 全国34省级三网延迟 (%d 点)", total))
	out := make([]provinceLatencyResult, 0, total)
	var mu sync.Mutex
	var done atomic.Int32
	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)
	for _, prov := range chinaProvinces {
		for _, carrier := range chinaCarrierNames {
			prov, carrier := prov, carrier
			ips := targets.ProvinceIPs(prov.Code, carrier)
			wg.Add(1)
			go func() {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				host, ms, text, loss := probeChinaTargetFailover(ctx, ips)
				lossText := ""
				if loss >= 0 {
					lossText = fmt.Sprintf("丢%.0f%%", loss)
				}
				mu.Lock()
				out = append(out, provinceLatencyResult{
					Code: prov.Code, Short: prov.Short, Name: prov.Name,
					Carrier: carrier, Host: host, Ms: ms, Text: text,
					Loss: loss, LossText: lossText,
				})
				n := done.Add(1)
				if n == int32(total) || n%15 == 0 {
					log(fmt.Sprintf("ChinaLatency: 进度 %d/%d", n, total))
				}
				mu.Unlock()
			}()
		}
	}
	wg.Wait()
	sort.Slice(out, func(i, j int) bool {
		if out[i].Code != out[j].Code {
			return provinceOrder(out[i].Code) < provinceOrder(out[j].Code)
		}
		return carrierOrder(out[i].Carrier) < carrierOrder(out[j].Carrier)
	})
	return out
}

func provinceOrder(code string) int {
	for i, p := range chinaProvinces {
		if p.Code == code {
			return i
		}
	}
	return 999
}

func carrierOrder(name string) int {
	order := map[string]int{"电信": 0, "联通": 1, "移动": 2}
	return order[name]
}

func chinaLatencySummary(results []provinceLatencyResult) (success, total int) {
	total = len(results)
	for _, r := range results {
		if r.Ms > 0 {
			success++
		}
	}
	return success, total
}
