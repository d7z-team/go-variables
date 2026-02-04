# go variables

> 简易的 golang 变量处理工具，支持从多种格式加载配置并提供统一的访问接口。

## 📦 安装

```bash
go get gopkg.d7z.net/go-variables
```

## ✨ 特性

- [x] 支持 **YAML** 格式解析 (`.yaml`, `.yml`)
- [x] 支持 **Properties** 格式解析 (`.properties`, `.prop`)
- [x] 支持 **XML** 格式解析 (`.xml`) (New!)
- [x] 支持嵌套结构访问 (`root.nested.key`)
- [x] 支持数组/列表索引访问 (`list.0`)
- [x] 支持变量模板与表达式

## 🚀 快速开始

```go
package main

import (
	"fmt"
	"gopkg.d7z.net/go-variables"
)

func main() {
	// 1. 创建变量容器
	v := variables.NewVariables()

	// 2. 加载配置 (示例：从 YAML 字符串)
	yamlData := `
app:
  name: demo
  version: 1.0
list:
  - item1
  - item2
`
	_ = v.FromYaml(yamlData, "")

	// 3. 加载配置 (示例：从文件)
	// _ = v.FromFile("config.xml", "")
    
    // 4. 设置值
    _ = v.Set("custom.key", "value")

	// 5. 获取值
	fmt.Println("Name:", v.Get("app.name"))       // Output: demo
	fmt.Println("Item 1:", v.Get("list.0"))       // Output: item1
    fmt.Println("Custom:", v.Get("custom.key"))   // Output: value
}
```

## 📄 许可证

此项目使用 [MIT](./LICENSE) 许可证。
