package encoding

import (
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
)

// ConvertToUTF8 将可能的GBK/Big5编码转换为UTF-8
func ConvertToUTF8(text string) string {
	if text == "" {
		return text
	}

	// 检查是否已经是有效的UTF-8
	if utf8.ValidString(text) && !hasGarbledChars(text) {
		return text
	}

	// 尝试GBK解码
	if decoded, err := simplifiedchinese.GBK.NewDecoder().String(text); err == nil {
		if utf8.ValidString(decoded) && !hasGarbledChars(decoded) {
			return decoded
		}
	}

	// 尝试Big5解码（繁体中文）
	if decoded, err := traditionalchinese.Big5.NewDecoder().String(text); err == nil {
		if utf8.ValidString(decoded) && !hasGarbledChars(decoded) {
			return decoded
		}
	}

	// 如果都失败，返回原文本
	return text
}

// hasGarbledChars 检查是否包含乱码特征
func hasGarbledChars(text string) bool {
	return strings.Contains(text, "�") || strings.Contains(text, "□")
}

// SafeConvert 安全转换文本
func SafeConvert(text interface{}) string {
	if text == nil {
		return ""
	}

	str, ok := text.(string)
	if !ok {
		return ""
	}

	return ConvertToUTF8(str)
}
