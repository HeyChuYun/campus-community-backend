package solver

import "testing"

const sampleCaptchaBase64 = "iVBORw0KGgoAAAANSUhEUgAAAGQAAAAZCAYAAADHXotLAAAB8UlEQVR4Xu2Xz0sCQRTH65D0g7oGBRYVBAX9AR26BF27BEIEgdAh6BD9Z9ElukRFXSpUPBRkamiZQqTuqrW/Xm+WlGV0zdWZnIX5wJeBneflfXgz4wBIhGKA/iDpL1KIYEghgiGFCEZDSOD2qWX8xqBWbBm/0HchRx+bdlhBi/C9kP+GlxC/IoUIRpOQlfgLTNwnYO0hA1dK1VnLBV5ClvQyjOC6qitwael0WU+ED2sdxytNQpwZv0tAtPLlrGcOLyHODGMilkGXdg3d9HbxSkPIbvIdYth8xTTholyF5XjalhJK5Jz1PVFvfqfphh2jAlFsvgIWnONkLOKkEClb+N0PuP4PuVFqtpDpSJLe6hq64X+FBdcohwiZ1Er0lpC4ClFxUoiQUTy2eMKy+a1QcVKIkCGGFz19LLWLV1yFkGOLCAlGU/QWU3gLIccWETLFcELopreLV2whG49ZOCtVoKAbUDRMOPlUYT6WsoXspfL0b5jCUsi6rsIpCsjjVBQxx6YGM793SNjg/2JkgS2Efl3VQ6Yj+832yUjDUgj9uqqHTEfGMulyIbGFkAt8+zkHczgVY3hnLOB6kC7Am8ZXBoGlEHKBh/A1FcSpCKCIWVz3cTJefSKD4HqHSPqDFCIYUohgSCGCIYUIhhQiGFKIYPwAgNj/1KkkzLUAAAAASUVORK5CYII="
const multiplyCaptchaBase64 = "iVBORw0KGgoAAAANSUhEUgAAAGQAAAAZCAYAAADHXotLAAACN0lEQVR4Xu2Y3UobQRTHfQ5fomBfoVAfwGvvVWjVFtPWpo1ENGKJXyi90OaikaaYUDERxQvJVUSrJQoNRPCLrKlpjBSbYJca/nUGGtZTG5q4uzna+cFhcmbP3OyPmTPZOihYUUcnFLVFCWGGEsIMJYQZJSGFPR37vhzWmw+wfDeJ5Yaksc5UHtW30ilTOPxyhNmFRXhGx/DY1YMO5wv0DL6CPxiElk7TcpaUhAgBNMzE3/oG6YQmf/8WInJ/m89QdT1auhx/jbYnT/ExHqdL2FESstq0h93Xx8jv6JYIWXu/AtcdBzYjn6QQMYp8LRCjpVUjdkY0FkPqYjfouo6zsx/YT6Uw6Z+WUrr7PXQJO67sIVYIEXzdycB7v18KEaPI7eB7Pi+FdL500UfssFVIdjeDoUaPFCJGkVuJ2CWid/jeBaSQwIdZWlIVse17/xyVYpuQ9ZnVS0dWPLwhczFvNrR/OD0DWIpGUSwWaWlV0JdeLirFNiFvW6b+aOqHnzXZ7M2GCml/7sTMXBg/z89pKTtsE2LEqmsvJV8oIJHcRt/wiBQTDEdoCTtqIsRujk9OpBCHu5c+qgp6LJWLSvkvhGSyWSnk4cXRZQb0pZeLSrlVQgbHJ7CxuYVsLif7hbjubiUScHu9UsjY5BRdwo6y/9SNcROgzdwY4lNK+sjaa7YZ3CohB5qG6VBIfr968KxbHlFu7xBCkXl8Oz2l5Sy58shS1A4lhBlKCDOUEGYoIcxQQpihhDDjF+Sq/Ai/XrepAAAAAElFTkSuQmCC"

// TestSolveBase64 验证加法验证码样例可以被正确识别。
func TestSolveBase64(t *testing.T) {
	result, err := SolveBase64(sampleCaptchaBase64)
	if err != nil {
		t.Fatalf("SolveBase64 returned error: %v", err)
	}

	if result.Expression == "" {
		t.Fatal("expression should not be empty")
	}

	if result.Code != "10" {
		t.Fatalf("unexpected code: got %s want 10, expression=%s", result.Code, result.Expression)
	}
}

// TestSolveMultiplyBase64 验证乘法验证码样例可以被正确识别。
func TestSolveMultiplyBase64(t *testing.T) {
	result, err := SolveBase64(multiplyCaptchaBase64)
	if err != nil {
		t.Fatalf("SolveBase64 returned error: %v", err)
	}

	if result.Expression == "" {
		t.Fatal("expression should not be empty")
	}

	if result.Code != "3" {
		t.Fatalf("unexpected code: got %s want 3, expression=%s", result.Code, result.Expression)
	}
}
