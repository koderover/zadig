package language

import (
	"context"
	"testing"
)

func TestSystemLanguage(t *testing.T) {
	for _, input := range []struct {
		value string
		want  Language
		tag   string
	}{
		{"", Chinese, "zh-CN"}, {"zh-CN", Chinese, "zh-CN"},
		{" EN-US ", English, "en-US"}, {"English", English, "en-US"},
		{"fr-fr", Chinese, "zh-CN"},
	} {
		lang := Normalize(input.value)
		if lang != input.want || lang.Tag() != input.tag || FromContext(WithContext(context.Background(), input.value)) != input.want {
			t.Fatalf("语言解析不符: %q", input.value)
		}
		if (lang.Text("中文", "English") == "English") != (lang == English) {
			t.Fatal("未按语言选择文案")
		}
	}
	if FromContext(context.Background()) != Chinese {
		t.Fatal("缺少上下文语言时须默认中文")
	}
}
