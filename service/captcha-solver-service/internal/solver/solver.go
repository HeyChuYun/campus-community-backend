package solver

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"sort"
	"strings"
)

// SolveResult 表示验证码识别与算式求值后的结果。
type SolveResult struct {
	Expression string `json:"expression"`
	Code       string `json:"code"`
}

// SolveBase64 负责把 base64 图片解码后，继续走识别和求值流程。
func SolveBase64(imageBase64 string) (*SolveResult, error) {
	// 第一步：去掉 data url 前缀，只保留纯 base64 数据。
	raw := strings.TrimSpace(imageBase64)
	if raw == "" {
		return nil, errors.New("image_base64 is empty")
	}
	if index := strings.Index(raw, ","); index >= 0 {
		raw = raw[index+1:]
	}

	// 第二步：把 base64 还原成图片字节。
	data, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("decode base64 failed: %w", err)
	}

	// 第三步：继续走图片识别。
	return SolveBytes(data)
}

// SolveBytes 负责对验证码图片做分割、模板匹配和算式求值。
func SolveBytes(data []byte) (*SolveResult, error) {
	// 第一步：解码图片。
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode image failed: %w", err)
	}

	// 第二步：把彩色验证码转成前景二值图，只保留非白色字符。
	mask := buildForegroundMask(img)

	// 第三步：提取连通域，得到每个字符的大致区域。
	components := extractComponents(mask)
	components = filterTinyComponents(components)
	components = mergeFragmentedComponents(components)
	if len(components) < 3 {
		return nil, fmt.Errorf("not enough captcha glyphs: %d", len(components))
	}

	// 第四步：按从左到右识别字符。
	sort.Slice(components, func(i, j int) bool {
		return components[i].minX < components[j].minX
	})

	var builder strings.Builder
	for _, component := range components {
		glyph := cropGlyph(mask, component)
		token, err := recognizeGlyph(glyph)
		if err != nil {
			return nil, err
		}
		builder.WriteString(token)
	}

	// 第五步：对识别出的算式求值，返回学校接口需要的验证码结果。
	expression := builder.String()
	code, err := evaluateExpression(expression)
	if err != nil {
		return nil, fmt.Errorf("evaluate expression %q failed: %w", expression, err)
	}

	return &SolveResult{
		Expression: expression,
		Code:       code,
	}, nil
}

type glyphComponent struct {
	minX   int
	minY   int
	maxX   int
	maxY   int
	pixels [][2]int
}

func (g glyphComponent) width() int {
	return g.maxX - g.minX + 1
}

func (g glyphComponent) height() int {
	return g.maxY - g.minY + 1
}

// buildForegroundMask 把图片转换成前景布尔矩阵。
func buildForegroundMask(img image.Image) [][]bool {
	bounds := img.Bounds()
	mask := make([][]bool, bounds.Dy())
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		row := make([]bool, bounds.Dx())
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r16, g16, b16, a16 := img.At(x, y).RGBA()
			if a16 == 0 {
				continue
			}

			r := uint8(r16 >> 8)
			g := uint8(g16 >> 8)
			b := uint8(b16 >> 8)

			if isForeground(r, g, b) {
				row[x-bounds.Min.X] = true
			}
		}
		mask[y-bounds.Min.Y] = row
	}
	return mask
}

// isForeground 根据颜色判断像素是否属于验证码字符。
func isForeground(r, g, b uint8) bool {
	maxValue := maxInt(int(r), int(g), int(b))
	minValue := minInt(int(r), int(g), int(b))
	avg := (int(r) + int(g) + int(b)) / 3

	// 白色或接近白色的背景直接过滤。
	if avg >= 248 && maxValue-minValue <= 8 {
		return false
	}

	// 彩色字符通常饱和度更高，即使整体偏亮也保留。
	if maxValue-minValue >= 20 {
		return true
	}

	// 对较深的灰色字符做兜底保留。
	return avg <= 225
}

// extractComponents 使用四连通算法把字符切分出来。
func extractComponents(mask [][]bool) []glyphComponent {
	if len(mask) == 0 || len(mask[0]) == 0 {
		return nil
	}

	height := len(mask)
	width := len(mask[0])
	visited := make([][]bool, height)
	for i := range visited {
		visited[i] = make([]bool, width)
	}

	var components []glyphComponent
	directions := [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if !mask[y][x] || visited[y][x] {
				continue
			}

			queue := [][2]int{{x, y}}
			visited[y][x] = true
			component := glyphComponent{
				minX: x,
				minY: y,
				maxX: x,
				maxY: y,
			}

			for len(queue) > 0 {
				current := queue[0]
				queue = queue[1:]
				component.pixels = append(component.pixels, current)
				component.minX = minInt(component.minX, current[0])
				component.minY = minInt(component.minY, current[1])
				component.maxX = maxInt(component.maxX, current[0])
				component.maxY = maxInt(component.maxY, current[1])

				for _, direction := range directions {
					nx := current[0] + direction[0]
					ny := current[1] + direction[1]
					if nx < 0 || ny < 0 || nx >= width || ny >= height {
						continue
					}
					if visited[ny][nx] || !mask[ny][nx] {
						continue
					}
					visited[ny][nx] = true
					queue = append(queue, [2]int{nx, ny})
				}
			}

			components = append(components, component)
		}
	}

	return components
}

// filterTinyComponents 去掉明显噪点。
func filterTinyComponents(components []glyphComponent) []glyphComponent {
	result := make([]glyphComponent, 0, len(components))
	for _, component := range components {
		if len(component.pixels) < 6 {
			continue
		}
		if component.width() <= 1 || component.height() <= 1 {
			continue
		}
		result = append(result, component)
	}
	return result
}

// mergeFragmentedComponents 用于把类似 "=" 这种拆成多个连通域的字符重新合并。
func mergeFragmentedComponents(components []glyphComponent) []glyphComponent {
	if len(components) <= 1 {
		return components
	}

	sort.Slice(components, func(i, j int) bool {
		if components[i].minX == components[j].minX {
			return components[i].minY < components[j].minY
		}
		return components[i].minX < components[j].minX
	})

	merged := make([]glyphComponent, 0, len(components))
	used := make([]bool, len(components))
	for i := 0; i < len(components); i++ {
		if used[i] {
			continue
		}

		current := components[i]
		used[i] = true

		for j := i + 1; j < len(components); j++ {
			if used[j] {
				continue
			}
			if shouldMergeComponent(current, components[j]) {
				current = mergeTwoComponents(current, components[j])
				used[j] = true
			}
		}

		merged = append(merged, current)
	}

	sort.Slice(merged, func(i, j int) bool {
		return merged[i].minX < merged[j].minX
	})
	return merged
}

func shouldMergeComponent(a, b glyphComponent) bool {
	xOverlap := minInt(a.maxX, b.maxX) - maxInt(a.minX, b.minX) + 1
	if xOverlap <= 0 {
		return false
	}

	minWidth := minInt(a.width(), b.width())
	if minWidth <= 0 {
		return false
	}

	overlapRatio := float64(xOverlap) / float64(minWidth)
	if overlapRatio < 0.6 {
		return false
	}

	verticalGap := 0
	if a.maxY < b.minY {
		verticalGap = b.minY - a.maxY
	} else if b.maxY < a.minY {
		verticalGap = a.minY - b.maxY
	}

	return verticalGap <= maxInt(6, minInt(a.height(), b.height())*2)
}

func mergeTwoComponents(a, b glyphComponent) glyphComponent {
	merged := glyphComponent{
		minX:   minInt(a.minX, b.minX),
		minY:   minInt(a.minY, b.minY),
		maxX:   maxInt(a.maxX, b.maxX),
		maxY:   maxInt(a.maxY, b.maxY),
		pixels: make([][2]int, 0, len(a.pixels)+len(b.pixels)),
	}
	merged.pixels = append(merged.pixels, a.pixels...)
	merged.pixels = append(merged.pixels, b.pixels...)
	return merged
}

// cropGlyph 把单个字符裁出来。
func cropGlyph(mask [][]bool, component glyphComponent) [][]bool {
	width := component.width()
	height := component.height()
	result := make([][]bool, height)
	for y := 0; y < height; y++ {
		result[y] = make([]bool, width)
	}

	for _, pixel := range component.pixels {
		result[pixel[1]-component.minY][pixel[0]-component.minX] = true
	}
	return result
}

// recognizeGlyph 把单个字符与内置模板比对，输出最接近的字符。
func recognizeGlyph(glyph [][]bool) (string, error) {
	normalized := normalizeGlyph(glyph, 12, 18)
	bestToken := ""
	bestScore := math.MaxFloat64

	for _, template := range builtinTemplates {
		score := compareGlyphs(normalized, template.mask)
		if score < bestScore {
			bestScore = score
			bestToken = template.token
		}
	}

	if bestToken == "" {
		return "", errors.New("recognize glyph failed")
	}
	return bestToken, nil
}

// normalizeGlyph 把字符统一缩放到固定尺寸，方便与模板比较。
func normalizeGlyph(glyph [][]bool, targetWidth, targetHeight int) [][]bool {
	sourceHeight := len(glyph)
	sourceWidth := len(glyph[0])
	result := make([][]bool, targetHeight)

	for y := 0; y < targetHeight; y++ {
		result[y] = make([]bool, targetWidth)
		sourceY := int(float64(y) * float64(sourceHeight) / float64(targetHeight))
		if sourceY >= sourceHeight {
			sourceY = sourceHeight - 1
		}
		for x := 0; x < targetWidth; x++ {
			sourceX := int(float64(x) * float64(sourceWidth) / float64(targetWidth))
			if sourceX >= sourceWidth {
				sourceX = sourceWidth - 1
			}
			result[y][x] = glyph[sourceY][sourceX]
		}
	}

	return result
}

// compareGlyphs 计算输入字符与模板之间的差异度，越小越接近。
func compareGlyphs(input, template [][]bool) float64 {
	resizedTemplate := normalizeGlyph(template, len(input[0]), len(input))
	total := 0.0
	for y := 0; y < len(input); y++ {
		for x := 0; x < len(input[y]); x++ {
			if input[y][x] != resizedTemplate[y][x] {
				total++
			}
		}
	}

	fillDiff := math.Abs(fillRatio(input) - fillRatio(resizedTemplate))
	return total + fillDiff*20
}

func fillRatio(mask [][]bool) float64 {
	total := 0
	filled := 0
	for y := range mask {
		for x := range mask[y] {
			total++
			if mask[y][x] {
				filled++
			}
		}
	}
	if total == 0 {
		return 0
	}
	return float64(filled) / float64(total)
}

// evaluateExpression 目前按“单个数字 + 运算符 + 单个数字”的学校验证码格式求值。
func evaluateExpression(expression string) (string, error) {
	expression = strings.TrimSpace(expression)
	expression = strings.TrimSuffix(expression, "=")
	expression = strings.TrimSuffix(expression, "?")

	if len(expression) < 3 {
		return "", errors.New("expression too short")
	}

	left := expression[0]
	op := expression[1]
	right := expression[2]
	if left < '0' || left > '9' || right < '0' || right > '9' {
		return "", fmt.Errorf("unsupported expression: %s", expression)
	}

	a := int(left - '0')
	b := int(right - '0')
	result := 0

	switch op {
	case '+':
		result = a + b
	case '-':
		result = a - b
	case 'x', 'X', '*':
		result = a * b
	default:
		return "", fmt.Errorf("unsupported operator: %c", op)
	}

	return fmt.Sprintf("%d", result), nil
}

type glyphTemplate struct {
	token string
	mask  [][]bool
}

var builtinTemplates = []glyphTemplate{
	newTemplate("5", []string{
		" ###### ",
		" ###### ",
		" ##     ",
		" ##     ",
		" ###### ",
		" #######",
		" ## ####",
		"     ###",
		"      ##",
		"     ###",
		" ## ####",
		"########",
		"####### ",
	}),
	newTemplate("=", []string{
		"###########",
		"###########",
		"           ",
		"           ",
		"###########",
		"###########",
	}),
	newTemplate("0", []string{
		" ### ",
		"#   #",
		"#  ##",
		"# # #",
		"##  #",
		"#   #",
		" ### ",
	}),
	newTemplate("1", []string{
		"  #  ",
		" ##  ",
		"# #  ",
		"  #  ",
		"  #  ",
		"  #  ",
		"#####",
	}),
	newTemplate("2", []string{
		" ### ",
		"#   #",
		"    #",
		"   # ",
		"  #  ",
		" #   ",
		"#####",
	}),
	newTemplate("3", []string{
		"#### ",
		"    #",
		"    #",
		" ### ",
		"    #",
		"    #",
		"#### ",
	}),
	newTemplate("4", []string{
		"#   #",
		"#   #",
		"#   #",
		"#####",
		"    #",
		"    #",
		"    #",
	}),
	newTemplate("5", []string{
		"#####",
		"#    ",
		"#    ",
		"#### ",
		"    #",
		"    #",
		"#### ",
	}),
	newTemplate("6", []string{
		" ### ",
		"#    ",
		"#    ",
		"#### ",
		"#   #",
		"#   #",
		" ### ",
	}),
	newTemplate("7", []string{
		"#####",
		"    #",
		"   # ",
		"  #  ",
		" #   ",
		" #   ",
		" #   ",
	}),
	newTemplate("8", []string{
		" ### ",
		"#   #",
		"#   #",
		" ### ",
		"#   #",
		"#   #",
		" ### ",
	}),
	newTemplate("9", []string{
		" ### ",
		"#   #",
		"#   #",
		" ####",
		"    #",
		"    #",
		" ### ",
	}),
	newTemplate("+", []string{
		"     ",
		"  #  ",
		"  #  ",
		"#####",
		"  #  ",
		"  #  ",
		"     ",
	}),
	newTemplate("-", []string{
		"     ",
		"     ",
		"     ",
		"#####",
		"     ",
		"     ",
		"     ",
	}),
	newTemplate("=", []string{
		"     ",
		"#####",
		"     ",
		"     ",
		"#####",
		"     ",
		"     ",
	}),
	newTemplate("x", []string{
		"#   #",
		" # # ",
		"  #  ",
		" # # ",
		"#   #",
		"     ",
		"     ",
	}),
	newTemplate("*", []string{
		"   #   ",
		"## # ##",
		"#######",
		"  ###  ",
		"  ###  ",
		"#######",
		"## # ##",
		"   #   ",
	}),
}

func newTemplate(token string, lines []string) glyphTemplate {
	mask := make([][]bool, len(lines))
	for y, line := range lines {
		mask[y] = make([]bool, len(line))
		for x, char := range line {
			mask[y][x] = char != ' '
		}
	}
	return glyphTemplate{
		token: token,
		mask:  mask,
	}
}

func maxInt(values ...int) int {
	result := values[0]
	for _, value := range values[1:] {
		if value > result {
			result = value
		}
	}
	return result
}

func minInt(values ...int) int {
	result := values[0]
	for _, value := range values[1:] {
		if value < result {
			result = value
		}
	}
	return result
}
