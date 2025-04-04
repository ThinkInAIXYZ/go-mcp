// MySQL MCP Server - 提供MySQL数据库访问工具
//
// 使用方法:
//
//	mysql_mcp_server -dsn "用户名:密码@tcp(主机:端口)/数据库名"
//
// 支持的工具:
//   - mysql_query: 执行MySQL查询（只读，SELECT语句）
//   - mysql_execute: 执行MySQL更新操作（非查询语句）
package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"

	"github.com/ThinkInAIXYZ/go-mcp/protocol"
	"github.com/ThinkInAIXYZ/go-mcp/server"
	"github.com/ThinkInAIXYZ/go-mcp/transport"
	_ "github.com/go-sql-driver/mysql"
)

// dsn 定义MySQL数据库连接字符串
// 格式: [username[:password]@][protocol[(address)]]/dbname[?param1=value1&...&paramN=valueN]
// 默认值: root:password@tcp(127.0.0.1:3306)/test
var dsn = flag.String("dsn", "root:password@tcp(127.0.0.1:3306)/test", "MySQL连接字符串")

// 数据库连接
var db *sql.DB

func main() {
	flag.Parse()

	// 初始化数据库
	if err := initDB(); err != nil {
		log.Fatalf("数据库连接失败: %v", err)
	}
	defer db.Close()

	// 创建MCP服务器
	srv, err := server.NewServer(
		transport.NewStdioServerTransport(),
		server.WithServerInfo(protocol.Implementation{
			Name:    "mysql-mcp-server",
			Version: "1.0.0",
		}),
	)
	if err != nil {
		log.Fatalf("服务器创建失败: %v", err)
	}

	// 注册查询工具
	srv.RegisterTool(&protocol.Tool{
		Name:        "mysql_query",
		Description: "执行MySQL查询（只读，SELECT语句）",
		InputSchema: protocol.InputSchema{
			Type: protocol.Object,
			Properties: map[string]interface{}{
				"sql": map[string]string{
					"type":        "string",
					"description": "要执行的SQL查询语句",
				},
			},
			Required: []string{"sql"},
		},
	}, handleQuery)

	// 注册执行工具
	srv.RegisterTool(&protocol.Tool{
		Name:        "mysql_execute",
		Description: "执行MySQL更新操作（INSERT/UPDATE/DELETE等非查询语句）",
		InputSchema: protocol.InputSchema{
			Type: protocol.Object,
			Properties: map[string]interface{}{
				"sql": map[string]string{
					"type":        "string",
					"description": "要执行的SQL语句",
				},
			},
			Required: []string{"sql"},
		},
	}, handleExecute)

	// 启动服务器
	log.Println("使用stdio传输模式启动MySQL MCP服务器")
	if err = srv.Run(); err != nil {
		log.Fatalf("服务运行错误: %v", err)
	}
}

// 初始化数据库连接
func initDB() error {
	var err error
	db, err = sql.Open("mysql", *dsn)
	if err != nil {
		return err
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(60) // 1分钟

	return db.Ping()
}

// 处理Mysql查询请求
func handleQuery(request *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
	sql, ok := request.Arguments["sql"].(string)
	if !ok {
		return nil, errors.New("sql必须是字符串")
	}

	// 执行查询
	rows, err := db.Query(sql)
	if err != nil {
		return nil, fmt.Errorf("查询执行错误: %v", err)
	}
	defer rows.Close()

	// 获取列名
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("获取列名失败: %v", err)
	}

	// 处理结果
	var results []map[string]interface{}
	values := make([]interface{}, len(columns))
	scanArgs := make([]interface{}, len(columns))
	for i := range values {
		scanArgs[i] = &values[i]
	}

	for rows.Next() {
		if err = rows.Scan(scanArgs...); err != nil {
			return nil, fmt.Errorf("读取行数据失败: %v", err)
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			if b, ok := val.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		results = append(results, row)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历结果错误: %v", err)
	}

	// 转换为JSON
	jsonData, err := json.Marshal(results)
	if err != nil {
		return nil, fmt.Errorf("JSON序列化失败: %v", err)
	}

	return &protocol.CallToolResult{
		Content: []protocol.Content{
			protocol.TextContent{
				Type: "text",
				Text: string(jsonData),
			},
		},
	}, nil
}

// 处理Mysql执行请求
func handleExecute(request *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
	sql, ok := request.Arguments["sql"].(string)
	if !ok {
		return nil, errors.New("sql必须是字符串")
	}

	// 执行SQL
	result, err := db.Exec(sql)
	if err != nil {
		return nil, fmt.Errorf("执行SQL错误: %v", err)
	}

	// 获取结果
	lastInsertID, _ := result.LastInsertId()
	rowsAffected, _ := result.RowsAffected()

	response := map[string]interface{}{
		"lastInsertId": lastInsertID,
		"rowsAffected": rowsAffected,
	}

	jsonData, err := json.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("JSON序列化失败: %v", err)
	}

	return &protocol.CallToolResult{
		Content: []protocol.Content{
			protocol.TextContent{
				Type: "text",
				Text: string(jsonData),
			},
		},
	}, nil
}
