
<a name="v0.1.6"></a>
## [v0.1.6](https://github.com/ThinkInAIXYZ/go-mcp/compare/v0.1.5...v0.1.6) (2025-04-11)

### Feat

*  update wechat_qrcode image (6fd298c)
*  add client to server Heartbeat (1141f58)
*  add server to client Heartbeat (7b04354)


<a name="v0.1.5"></a>
## [v0.1.5](https://github.com/ThinkInAIXYZ/go-mcp/compare/v0.1.4...v0.1.5) (2025-04-11)

### Fix

*  Add Debug Logger to display debug information (65a3ad8)
*  Avoid service exit due to incorrect input formats in stdio (36cc48d)


<a name="v0.1.4"></a>
## [v0.1.4](https://github.com/ThinkInAIXYZ/go-mcp/compare/v0.1.3...v0.1.4) (2025-04-10)

### Docs

*  add star history (622d582)
* **protocol:**  add VerifyAndUnmarshal function (c1ad68c)

### Feat

* **protocol:**  add VerifyAndUnmarshal function (42a5071)


<a name="v0.1.3"></a>
## [v0.1.3](https://github.com/ThinkInAIXYZ/go-mcp/compare/v0.1.2...v0.1.3) (2025-04-09)

### Docs

*  reduce wechat code (44cead0)

### Examples

*  optimization (51cb740)

### Feat

*  readme add wechat qrcode (519f864)
*  readme add wechat qrcode (2b19574)
*  add feishu link (5481535)

### Fix

*  readme image link (c2458b4)

### Reverts

* feat: add feishu link


<a name="v0.1.2"></a>
## [v0.1.2](https://github.com/ThinkInAIXYZ/go-mcp/compare/v0.1.1...v0.1.2) (2025-04-09)

### Feat

*  optimization new tool (f05273e)


<a name="v0.1.1"></a>
## [v0.1.1](https://github.com/ThinkInAIXYZ/go-mcp/compare/v0.1.0...v0.1.1) (2025-04-08)

### Fix

*  json JsonUnmarshal UseInt64 true (0b4d0d9)

### Refactor

*  stdio_shutdown (4bc2908)


<a name="v0.1.0"></a>
## v0.1.0 (2025-04-04)

### Docs

*  update readme (f613144)
*  update readme (fdfe616)
*  update readme (be624d3)
*  design.md (8697968)
*  translate annotate and readme.md (ae7528f)
*  Readme.md add why to do go-mcp (20af48b)
*  add architecture design to Readme.md (c27606e)

### Fead

* **stdio_transport:**  update stdio transport client (8917ef0)

### Feat

*  perfecting the framework (5fcf7f7)
*  add pre-commit (ee5da68)
*  optimization tool struct (4e2ec36)
*  optimization test (e0be9cd)
*  init project framework (912b969)
*  modify test.yml (0156186)
*  downgrade go version (aa02032)
*  refactor ServerTransport writeError (298e2ab)
*  delete cursor (0e80394)
*  refactor ServerTransport close to shutdown (0f717d9)
*  client add capabilities check (d384dcf)
*  stdio add option (6727a78)
*  add (b3fd05c)
*  add annotate (1a6e06a)
*  replace server sessionID2session (998783c)
*  add server gracefully shut down logic (33046bc)
*  readme add contributors (15798b9)
*  sse start bug (cea2766)
*  solve conflict“ (cc374ca)
*  merge main (30779d6)
*  build part package (de069c0)
*  add request response matching (9fbd923)
*  add request response matching (9af60e6)
*  perfecting the framework (b7e62cd)
*  add e2e test (41daa6e)
*  perfecting the framework (15ff251)
*  add test and example (7a54b78)
*  add defer recover (315f5a8)
*  optimization desgin.md (df8b27d)
*  add logger (e8dace1)
*  build server package (debb86a)
*  build client package (f0b54d2)
* **stdio_transport:**  add stdio client/server transport impl (e4ee22e)
* **transport:**  sse transport (7e531b0)

### Fix

*  client test (ceef300)
*  empty param parse (1d000aa)
*  stdio server shutdown bug (28cd9db)
*  server and client test (5ccc35a)
*  stdio server run (52b31d0)
*  sse and client bug (2aaba60)
*  sse and server bug (f44e83f)
*  test (b43e3e6)
*  test (d5dd2ee)
*  test: (1cb8a97)
*  server test (0a2710e)
*  read resource (ba1d4dc)
*  ServerTransport interface (52e474c)
*  Prevent memory leaks in JSON-RPC client (0c7495c)
*  empty param parse (8d0b208)
*  transport test (a17e834)
*  Shutdown logic (539035c)
*  Shutdown cancel (f91a652)
*  receive some bug (5aa2bf0)
*  code conflict, Merge branch 'main' into feature/stdio (27a9a66)
*  some bug (a2897bc)
*  update test and sseClient ctx param name (3a2b7ba)
*  modify transport_test (773d4de)
*  modify transport_test and replace sync.Map with sessionStore in pkg (bd5eb93)
*  server listtools (9caede5)
* **stdio_transport:**  delete testdata (e0d6e04)

### Refactor

*  trasnport detail (6872e82)
*  server and client receive to not import (400b3af)
*  protocol response to result (0094f7c)
*  Simplified logging (e3987b5)
*  package name (4cb773e)
*  server handle (d8d7f66)
*  server Register (bdabd75)
*  server initialize (7acf660)
*  server receive (9c5f12d)
*  client receive (5a085d6)
*  part particulars (13c2073)
*  client call (1ba8a14)
*  client call (6b5245d)
*  server call (5b28da4)
*  stdio test (8df59dd)
*  stdio (61e25f6)
*  part particulars (a6884bd)
*  test logic (3bf8ebe)
*  pkg.JsonUnmarshal add error info (5ff76cc)

### Test

*  add TestServerNotify (6b59459)
*  add client_test (4f0a4f0)
*  add server_test (e39b373)

### Reverts

* Update call.go

