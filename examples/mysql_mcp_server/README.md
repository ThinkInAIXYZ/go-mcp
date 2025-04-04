# MySQL MCP Sever 例子

这是一个基于Go-MCP框架实现的MySQL工具服务器，提供了通过MCP协议与MySQL数据库交互的能力。

## 功能

服务器提供以下工具：

1. `mysql_query` - 执行MySQL查询（只读SELECT语句）
2. `mysql_execute` - 执行MySQL更新操作（非查询语句如INSERT/UPDATE/DELETE）

## 安装与运行

### 安装步骤

使用 `go install` 命令安装到 `$GOPATH/bin` 目录：

```bash
# 从GitHub克隆仓库
git clone https://github.com/ThinkInAIXYZ/go-mcp.git
cd go-mcp/examples/mysql_mcp_server

# 安装MySQL MCP服务器
go install
```

安装完成后，可以直接运行：

```bash
# 使用默认配置
mysql_mcp_server

# 指定MySQL连接
mysql_mcp_server -dsn "用户名:密码@tcp(主机:端口)/数据库名"
```

## 在Deepchat中配置与使用
### 配置步骤
1. 进入设置->MCP设置->添加服务器
2. 填入如下配置
```json
{
    "mcpServers": {
      "go_mysql_mcp": {
        "command": "mysql_mcp_server",
        "args": [
            "-dsn",
            "root:password@tcp(127.0.0.1:3306)/test"
        ]
      }
    }
}
```

## 在Cursor中配置与使用

### 配置步骤

1. 编辑Cursor的MCP配置文件（通常位于`~/.cursor/mcp.json`）
2. 添加如下配置：

```json
"go_mysql_mcp": {
  "command": "mysql_mcp_server",
  "args": [
    "-dsn",
    "root:password@tcp(127.0.0.1:3306)/test"
  ]
}
```

### 在Cursor中使用

直接在Cursor对话中使用：

```
/mysql_query SELECT * FROM users LIMIT 10
```

```
/mysql_execute INSERT INTO users (name, email) VALUES ('张三', 'zhangsan@example.com')
```