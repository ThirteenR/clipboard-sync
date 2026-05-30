# 设备别名功能设计文档

## 概述

为clipboard-sync应用添加设备别名功能，允许用户为设备设置可读的别名，提高设备发现时的可读性。别名仅用于展示，最终映射仍以UUID为准。

## 设计决策

### 1. 触发条件

**选择：配置文件检查**
- 每次启动时检查 `~/.config/clipboardsync/trusted.json` 中的 `device_alias` 字段
- 如果字段为空，提示用户设置别名
- 优势：可靠持久化，不依赖临时文件

### 2. 存储位置

**选择：扩展trusted.json**
- 在现有配置文件中添加两个新字段：
  - `device_alias`：当前设备的别名（字符串）
  - `device_aliases`：其他设备的UUID→别名映射（对象）

### 3. 显示方式

**选择：heartbeat携带alias + 本地存储**
- 设备别名在组播heartbeat消息中广播
- 其他设备收到heartbeat后存储UUID→别名映射
- 实时更新，无需额外查询

### 4. 修改方式

**选择：交互式TUI + 命令行参数**
- `clipboardsync alias`：进入交互式TUI
- `clipboardsync alias set <别名>`：直接设置别名
- `clipboardsync alias show`：显示当前别名

### 5. 限制要求

**长度限制：** 最大20字符

**字符限制：**
- 允许：中文、英文、数字、空格、连字符(-)、下划线(_)、点号(.)
- 禁止：控制字符、特殊符号（如 `\n`, `\t`, `\0`、`@`、`#`、`$`、`%`、`^`、`&`、`*`、`(`、`)`、`+`、`=`、`{`、`}`、`[`、`]`、`|`、`\`、`:`、`"`、`'`、`;`、`<`、`>`、`,`、`?`、`/`、`~`、`` ` ``）

**唯一性：** 不强制唯一性，允许重复别名

### 6. 显示格式

**统一格式：** `别名 (hostname)`

**示例：**
- `我的MacBook (MacBook-Pro)`
- `张三的电脑 (DESKTOP-ABC123)`
- `办公室电脑 (Office-PC)`

**特殊情况：**
- 没有别名：`hostname (UUID前8位)`
- 没有hostname：`别名 (UUID前8位)`
- 都没有：`UUID`

## 数据结构

### trusted.json 结构

```json
{
  "trusted_uuids": ["uuid1", "uuid2"],
  "devices": {
    "uuid1": {
      "hostname": "MacBook-Pro",
      "last_seen": "2024-01-01T00:00:00Z"
    }
  },
  "device_alias": "我的MacBook",
  "device_aliases": {
    "uuid2": "张三的电脑"
  }
}
```

### heartbeat消息结构

```go
type heartbeatMsg struct {
    UUID     string `json:"uuid"`
    Hostname string `json:"hostname"`
    Port     int    `json:"port"`
    Alias    string `json:"alias,omitempty"`  // 新增字段
}
```

## 接口设计

### trust包新增方法

```go
// 别名验证
func validateAlias(alias string) error

// 当前设备别名管理
func (ts *TrustStore) GetDeviceAlias() string
func (ts *TrustStore) SetDeviceAlias(alias string) error

// 其他设备别名管理
func (ts *TrustStore) GetPeerAlias(uuid string) string
func (ts *TrustStore) SetPeerAlias(uuid, alias string)

// 别名是否存在
func (ts *TrustStore) HasDeviceAlias() bool
```

### 错误类型

```go
var (
    ErrAliasTooLong     = errors.New("别名长度不能超过20字符")
    ErrAliasInvalidChar = errors.New("别名包含无效字符")
    ErrAliasEmpty       = errors.New("别名不能为空")
)
```

## 集成设计

### discovery流程修改

1. **发送heartbeat时：**
   - 从trustStore获取当前设备别名
   - 在heartbeatMsg中添加Alias字段

2. **接收heartbeat时：**
   - 如果消息包含Alias字段
   - 调用trustStore.SetPeerAlias(uuid, alias)存储映射

3. **显示设备时：**
   - 使用formatDisplayName(alias, hostname, uuid)函数
   - 优先显示别名，没有则显示hostname

### main.go修改

1. 在main()中检查别名是否存在
2. 如果不存在，提示用户设置
3. 在discovery注册时传递别名
4. 在日志中使用统一显示格式

## 交互设计

### TUI界面

```
=== Clipboard Sync 设备别名管理 ===
当前设备别名：我的MacBook
UUID：xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx

操作：
  [1] 修改别名
  [2] 查看其他设备别名
  [q] 退出
请选择：
```

### 命令行参数

```bash
clipboardsync alias          # 进入交互式TUI
clipboardsync alias set <别名>  # 直接设置别名
clipboardsync alias show      # 显示当前别名
```

### 验证流程

1. 长度检查：≤20字符
2. 字符检查：只允许中文、英文、数字、空格、常见标点
3. 确认提示：显示"设置别名为: XXX？(y/n)"

## 错误处理

### 验证策略

- **验证失败**：提示用户重新输入，显示具体错误原因
- **文件读写错误**：记录日志，继续运行（不阻塞主流程）
- **别名冲突**：不检查唯一性，允许重复（UUID仍是唯一标识）

### 日志记录

```go
log.Printf("设备别名设置为: %s", alias)
log.Printf("设备 %s 别名更新: %s", uuid, alias)
log.Printf("别名验证失败: %v", err)
```

### 默认行为

- **首次启动**：如果 `device_alias` 字段为空，使用hostname作为默认别名，提示用户确认或修改
- **用户取消设置**：保留当前别名（如果有），否则使用hostname
- **读取别名失败**：返回空字符串，使用hostname作为显示别名

## 测试策略

### 单元测试

1. 别名验证函数测试
2. 别名存储/读取测试
3. 别名映射关系测试

### 集成测试

1. heartbeat消息携带别名测试
2. 别名显示格式测试
3. TUI交互测试

## 实现步骤

1. 修改trust/store.go：添加数据结构和方法
2. 修改trust/tui.go：添加别名管理TUI
3. 修改discovery/multicast.go：集成别名到heartbeat
4. 修改main.go：添加别名检查和子命令
5. 添加单元测试
6. 更新文档

## 风险与缓解

### 风险1：数据迁移
- **风险**：现有trusted.json没有新字段
- **缓解**：使用omitempty标签，自动处理缺失字段

### 风险2：性能影响
- **风险**：heartbeat消息变大
- **缓解**：别名字段可选，通常很短

### 风险3：字符编码
- **风险**：中文字符在不同系统间传输
- **缓解**：使用UTF-8编码，JSON标准支持

## 成功标准

1. 用户能够设置和修改设备别名
2. 别名在设备发现时正确显示
3. 别名在heartbeat中正确传输和接收
4. 错误情况得到妥善处理
5. 不影响现有功能