package probe

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

type mediaCheckResult struct {
	Status        string
	Region        string
	UnlockType    string
	ScorePositive bool
}

func isPositiveMediaStatus(status string) bool {
	switch status {
	case "解锁", "自制剧", "仅网页", "仅APP":
		return true
	default:
		return false
	}
}

func checkMediaService(ctx context.Context, client *http.Client, name, url, publicIP string) mediaCheckResult {
	domain := mediaDomain(url)
	unlockType := ""
	if publicIP != "" {
		unlockType = dnsUnlockType(ctx, domain, publicIP)
	}

	var result mediaCheckResult
	switch name {
	case "Netflix":
		result = checkNetflix(ctx, client)
	case "Disney+":
		result = checkDisneyPlus(ctx, client)
	case "YouTube Premium":
		result = checkYouTubePremium(ctx, client)
	case "ChatGPT":
		result = checkChatGPT(ctx, client)
	case "Gemini":
		result = checkGemini(ctx, client)
	case "DAZN":
		result = checkDazn(ctx, client)
	case "BBC iPlayer":
		result = checkBBCiPlayer(ctx, client)
	case "TikTok":
		result = checkTikTok(ctx, client, url)
	case "Spotify":
		result = checkSpotify(ctx, client)
	case "Prime Video":
		result = checkPrimeVideo(ctx, client, url)
	default:
		result = checkGenericMedia(ctx, client, url)
	}
	result.UnlockType = unlockType
	return result
}

// checkNetflix 参照 netflix-verify / RegionRestrictionCheck：
// 双片源 + 重定向地区码，区分完整解锁与仅自制剧。
func checkNetflix(ctx context.Context, client *http.Client) mediaCheckResult {
	noRedirect := newMediaClient(client.Timeout, false)
	follow := newMediaClient(client.Timeout, true)

	const (
		areaTitle     = "81280792"
		nonSelfMade   = "70143836"
		regionTitle   = "80018499"
	)

	code1, _, h1, err1 := mediaGET(ctx, noRedirect, "https://www.netflix.com/title/"+areaTitle)
	code2, _, h2, err2 := mediaGET(ctx, noRedirect, "https://www.netflix.com/title/"+nonSelfMade)
	if err1 != nil && err2 != nil {
		return mediaCheckResult{Status: "失败"}
	}
	if code1 == 403 && code2 == 403 {
		return mediaCheckResult{Status: "封禁", Region: extractNetflixRegion(h1, h2)}
	}

	region := extractNetflixRegion(h1, h2)
	if region == "" {
		_, _, h3, _ := mediaGET(ctx, noRedirect, "https://www.netflix.com/title/"+regionTitle)
		region = extractNetflixRegion(h3, nil)
	}

	if code1 == 404 && code2 == 404 {
		return mediaCheckResult{Status: "自制剧", Region: region, ScorePositive: true}
	}

	_, body1, _, _ := mediaGET(ctx, follow, "https://www.netflix.com/title/"+areaTitle)
	_, body2, _, _ := mediaGET(ctx, follow, "https://www.netflix.com/title/"+nonSelfMade)
	ohNo1 := strings.Contains(body1, "Oh no!")
	ohNo2 := strings.Contains(body2, "Oh no!")
	if ohNo1 && ohNo2 {
		return mediaCheckResult{Status: "自制剧", Region: region, ScorePositive: true}
	}

	if code1 == 200 || code2 == 200 || serviceUnlocked(code1, body1) || serviceUnlocked(code2, body2) {
		if region == "" {
			region = extractNetflixRegionFromBody(body1 + body2)
		}
		return mediaCheckResult{Status: "解锁", Region: region, ScorePositive: true}
	}

	return mediaCheckResult{Status: "封禁", Region: region}
}

func extractNetflixRegion(h1, h2 http.Header) string {
	for _, h := range []http.Header{h1, h2} {
		if h == nil {
			continue
		}
		if region := netflixRegionFromURL(h.Get("x-originating-url")); region != "" {
			return region
		}
		if region := netflixRegionFromURL(h.Get("Location")); region != "" {
			return region
		}
	}
	return ""
}

func netflixRegionFromURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parts := strings.Split(raw, "/")
	if len(parts) <= 3 {
		return ""
	}
	region := strings.ToUpper(strings.Split(parts[3], "-")[0])
	if region == "TITLE" || region == "" {
		return "US"
	}
	if len(region) == 2 {
		return region
	}
	return ""
}

var netflixCountryRE = regexp.MustCompile(`"id"\s*:\s*"([A-Z]{2})"`)

func extractNetflixRegionFromBody(body string) string {
	if m := netflixCountryRE.FindStringSubmatch(body); len(m) == 2 {
		return m[1]
	}
	return extractRegion(body)
}

// checkDisneyPlus 参照 RegionRestrictionCheck 的 bamgrid API 流程。
func checkDisneyPlus(ctx context.Context, client *http.Client) mediaCheckResult {
	if result, ok := checkDisneyPlusAPI(ctx, client); ok {
		return result
	}
	return checkDisneyPlusPage(ctx, client)
}

func checkDisneyPlusAPI(ctx context.Context, client *http.Client) (mediaCheckResult, bool) {
	const bearer = "ZGlzbmV5JmJyb3dzZXImMS4wLjA.Cu56AgSfBTDag5NiRA81oLHkDZfu5L3CKadnefEAY84"
	authHeaders := map[string]string{"Authorization": "Bearer " + bearer}

	code, body, err := mediaPOST(ctx, client,
		"https://disney.api.edge.bamgrid.com/devices",
		"application/json; charset=UTF-8",
		`{"deviceFamily":"browser","applicationRuntime":"chrome","deviceProfile":"windows","attributes":{}}`,
		authHeaders,
	)
	if err != nil || code != 200 {
		return mediaCheckResult{}, false
	}

	var deviceResp struct {
		Assertion string `json:"assertion"`
	}
	if json.Unmarshal([]byte(body), &deviceResp) != nil || deviceResp.Assertion == "" {
		return mediaCheckResult{}, false
	}

	tokenBody := url.Values{
		"grant_type":         {"urn:ietf:params:oauth:grant-type:token-exchange"},
		"latitude":           {"0"},
		"longitude":          {"0"},
		"platform":           {"browser"},
		"subject_token":      {deviceResp.Assertion},
		"subject_token_type": {"urn:bamtech:params:oauth:token-type:device"},
	}.Encode()
	code, tokenResp, err := mediaPOST(ctx, client,
		"https://disney.api.edge.bamgrid.com/token",
		"application/x-www-form-urlencoded",
		tokenBody,
		authHeaders,
	)
	if err != nil {
		return mediaCheckResult{}, false
	}
	if code != 200 {
		if strings.Contains(tokenResp, "forbidden-location") || strings.Contains(tokenResp, "403 ERROR") {
			return mediaCheckResult{Status: "封禁"}, true
		}
		return mediaCheckResult{}, false
	}

	var tokenData struct {
		RefreshToken string `json:"refresh_token"`
	}
	if json.Unmarshal([]byte(tokenResp), &tokenData) != nil || tokenData.RefreshToken == "" {
		return mediaCheckResult{}, false
	}

	graphBody := fmt.Sprintf(
		`{"query":"mutation refreshToken($input: RefreshTokenInput!) { refreshToken(refreshToken: $input) { activeSession { sessionId } } }","variables":{"input":{"refreshToken":%q}}}`,
		tokenData.RefreshToken,
	)
	code, graphResp, err := mediaPOST(ctx, client,
		"https://disney.api.edge.bamgrid.com/graph/v1/device/graphql",
		"application/json",
		graphBody,
		map[string]string{"Authorization": bearer},
	)
	if err != nil || code != 200 {
		if strings.Contains(graphResp, "forbidden-location") {
			return mediaCheckResult{Status: "封禁"}, true
		}
		return mediaCheckResult{}, false
	}

	region := strings.ToUpper(strings.TrimSpace(jsonField(graphResp, "countryCode")))
	inSupported := strings.Contains(graphResp, `"inSupportedLocation":true`) ||
		strings.Contains(graphResp, `"inSupportedLocation": true`)

	if region != "" && inSupported {
		return mediaCheckResult{Status: "解锁", Region: region, ScorePositive: true}, true
	}
	if region != "" && !inSupported {
		return mediaCheckResult{Status: "封禁", Region: region}, true
	}
	if strings.Contains(graphResp, "forbidden-location") {
		return mediaCheckResult{Status: "封禁"}, true
	}
	return mediaCheckResult{}, false
}

func checkDisneyPlusPage(ctx context.Context, client *http.Client) mediaCheckResult {
	result := checkGenericMedia(ctx, client, "https://www.disneyplus.com/")
	if result.Status == "失败" {
		result.Status = "封禁"
	}
	return result
}

var youtubeCountryRE = regexp.MustCompile(`"countryCode"\s*:\s*"([A-Z]{2})"`)

func checkYouTubePremium(ctx context.Context, client *http.Client) mediaCheckResult {
	follow := newMediaClient(client.Timeout, true)
	ytHeaders := map[string]string{
		"Cookie":          "CONSENT=YES+cb.20220301-11-p0.en+FX+700; PREF=tz=UTC",
		"Accept-Language": "en",
	}
	code, body, _, err := mediaGETWithHeaders(ctx, follow, "https://www.youtube.com/premium", ytHeaders)
	if err != nil || code == 0 {
		return mediaCheckResult{Status: "失败"}
	}
	lower := strings.ToLower(body)
	if strings.Contains(body, "www.google.cn") {
		return mediaCheckResult{Status: "封禁", Region: "CN"}
	}
	if strings.Contains(lower, "premium is not available in your country") ||
		strings.Contains(lower, "not available in your country") {
		return mediaCheckResult{Status: "封禁"}
	}

	region := ""
	if m := youtubeCountryRE.FindStringSubmatch(body); len(m) == 2 {
		region = m[1]
	}
	if region == "" {
		region = extractRegion(body)
	}
	if region == "" {
		region = traceCountry(ctx, client)
	}

	unlockMarkers := []string{
		"ad-free", "youtube premium", "premium membership", "ytpoffer",
		"premium_plan", `"premium"`,
	}
	for _, marker := range unlockMarkers {
		if strings.Contains(lower, marker) {
			return mediaCheckResult{Status: "解锁", Region: region, ScorePositive: true}
		}
	}

	if code == 403 || code == 451 {
		return mediaCheckResult{Status: "封禁"}
	}
	if serviceUnlocked(code, body) {
		return mediaCheckResult{Status: "解锁", Region: region, ScorePositive: true}
	}
	return mediaCheckResult{Status: "封禁"}
}

func checkChatGPT(ctx context.Context, client *http.Client) mediaCheckResult {
	_, apiBody, _, apiErr := mediaGET(ctx, client, "https://api.openai.com/compliance/cookie_requirements")
	iosCode, iosBody, _, iosErr := mediaGET(ctx, client, "https://ios.chat.openai.com/")
	if apiErr != nil && iosErr != nil {
		return mediaCheckResult{Status: "失败"}
	}

	apiBlocked := strings.Contains(apiBody, "unsupported_country")
	iosBlocked := iosCode == 403 || strings.Contains(iosBody, "VPN")

	if apiBlocked {
		code, _, _, _ := mediaGET(ctx, client, "https://chatgpt.com/favicon.ico")
		if code != 403 {
			apiBlocked = false
		}
	}

	region := traceCountry(ctx, client)

	switch {
	case !apiBlocked && !iosBlocked:
		return mediaCheckResult{Status: "解锁", Region: region, ScorePositive: true}
	case apiBlocked && iosBlocked:
		return mediaCheckResult{Status: "封禁", Region: region}
	case !apiBlocked && iosBlocked:
		return mediaCheckResult{Status: "仅网页", Region: region, ScorePositive: true}
	case apiBlocked && !iosBlocked:
		return mediaCheckResult{Status: "仅APP", Region: region, ScorePositive: true}
	default:
		return mediaCheckResult{Status: "失败", Region: region}
	}
}

var geminiUnlockRE = regexp.MustCompile(`45631641,null,true`)
var geminiCountryRE = regexp.MustCompile(`,2,1,200,"([A-Z]{3})"`)

func checkGemini(ctx context.Context, client *http.Client) mediaCheckResult {
	follow := newMediaClient(client.Timeout, true)
	_, body, _, err := mediaGET(ctx, follow, "https://gemini.google.com/")
	if err != nil {
		return mediaCheckResult{Status: "失败"}
	}
	if !geminiUnlockRE.MatchString(body) {
		if strings.Contains(strings.ToLower(body), "not available") {
			return mediaCheckResult{Status: "封禁"}
		}
		return mediaCheckResult{Status: "封禁"}
	}
	region := ""
	if m := geminiCountryRE.FindStringSubmatch(body); len(m) == 2 {
		region = m[1]
	}
	return mediaCheckResult{Status: "解锁", Region: region, ScorePositive: true}
}

func checkDazn(ctx context.Context, client *http.Client) mediaCheckResult {
	payload := `{"LandingPageKey":"generic","Languages":"zh-CN,zh,en","Platform":"web","PlatformAttributes":{},"Manufacturer":"","PromoCode":"","Version":"2"}`
	code, body, err := mediaPOST(ctx, client,
		"https://startup.core.indazn.com/misl/v5/Startup",
		"application/json",
		payload,
		nil,
	)
	if err != nil || code == 0 {
		return mediaCheckResult{Status: "失败"}
	}
	if strings.Contains(body, `"isAllowed":true`) || strings.Contains(body, `"isAllowed": true`) {
		region := strings.ToUpper(jsonField(body, "GeolocatedCountry"))
		return mediaCheckResult{Status: "解锁", Region: region, ScorePositive: true}
	}
	if strings.Contains(body, `"isAllowed":false`) || strings.Contains(body, `"isAllowed": false`) {
		return mediaCheckResult{Status: "封禁"}
	}
	return mediaCheckResult{Status: "失败"}
}

func checkBBCiPlayer(ctx context.Context, client *http.Client) mediaCheckResult {
	follow := newMediaClient(client.Timeout, true)
	code, body, _, err := mediaGET(ctx, follow, "https://www.bbc.co.uk/iplayer")
	if err != nil {
		return mediaCheckResult{Status: "失败"}
	}
	lower := strings.ToLower(body)
	if strings.Contains(lower, "not available outside the uk") ||
		strings.Contains(lower, "this content is not available in your location") ||
		code == 403 {
		return mediaCheckResult{Status: "封禁"}
	}
	if serviceUnlocked(code, body) {
		return mediaCheckResult{Status: "解锁", Region: "GB", ScorePositive: true}
	}
	return mediaCheckResult{Status: "封禁"}
}

func checkTikTok(ctx context.Context, client *http.Client, url string) mediaCheckResult {
	result := checkGenericMedia(ctx, client, url)
	if result.ScorePositive {
		_, body, _, _ := mediaGET(ctx, client, url)
		if region := extractRegion(body); region != "" {
			result.Region = region
		}
	}
	return result
}

func checkSpotify(ctx context.Context, client *http.Client) mediaCheckResult {
	code, _, _, err := mediaGET(ctx, client, "https://www.spotify.com/api/content/v1/country")
	if err == nil && code == 200 {
		return mediaCheckResult{Status: "解锁", ScorePositive: true}
	}
	code, _, _, err = mediaGET(ctx, client, "https://www.spotify.com/us/account/overview/")
	if err != nil {
		return mediaCheckResult{Status: "失败"}
	}
	if code == 200 {
		region := traceCountry(ctx, client)
		return mediaCheckResult{Status: "解锁", Region: region, ScorePositive: true}
	}
	if code == 403 || code == 451 {
		return mediaCheckResult{Status: "封禁"}
	}
	return mediaCheckResult{Status: "封禁"}
}

func checkPrimeVideo(ctx context.Context, client *http.Client, url string) mediaCheckResult {
	follow := newMediaClient(client.Timeout, true)
	code, body, _, err := mediaGET(ctx, follow, url)
	if err != nil {
		return mediaCheckResult{Status: "失败"}
	}
	if code == 403 || code == 451 {
		return mediaCheckResult{Status: "封禁"}
	}
	if !serviceUnlocked(code, body) {
		return mediaCheckResult{Status: "封禁"}
	}
	region := extractRegion(body)
	for _, key := range []string{"currentTerritory", "currentCountryCode", "regionRedirectUrl"} {
		if region != "" {
			break
		}
		if idx := strings.Index(body, key); idx >= 0 {
			snippet := body[idx:min(idx+120, len(body))]
			if m := regexp.MustCompile(`"([A-Z]{2})"`).FindStringSubmatch(snippet); len(m) == 2 {
				region = m[1]
			}
		}
	}
	return mediaCheckResult{Status: "解锁", Region: region, ScorePositive: true}
}

func checkGenericMedia(ctx context.Context, client *http.Client, url string) mediaCheckResult {
	follow := newMediaClient(client.Timeout, true)
	code, body, _, err := mediaGET(ctx, follow, url)
	if err != nil {
		return mediaCheckResult{Status: "失败"}
	}
	if code == 403 || code == 451 {
		return mediaCheckResult{Status: "封禁"}
	}
	if serviceUnlocked(code, body) {
		return mediaCheckResult{Status: "解锁", Region: extractRegion(body), ScorePositive: true}
	}
	return mediaCheckResult{Status: "封禁"}
}

func jsonField(body, key string) string {
	idx := strings.Index(body, `"`+key+`"`)
	if idx < 0 {
		return ""
	}
	snippet := body[idx:min(idx+120, len(body))]
	parts := strings.Split(snippet, `"`)
	if len(parts) >= 4 {
		return parts[3]
	}
	return ""
}

func jsonPathString(v any, keys ...string) string {
	cur := v
	for _, key := range keys {
		m, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur, ok = m[key]
		if !ok {
			return ""
		}
	}
	s, _ := cur.(string)
	return s
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
