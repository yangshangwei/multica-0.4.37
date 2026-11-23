# 设计与实现边界

模式为 Impeccable Read：延续现有文档阅读栏、字阶与中性主题。图用清晰节点、短中文标签、箭头动词和明确分组，保留文字解释。操作图优先纵向或紧凑横向，架构按一张图一个层级，复杂时间顺序采用时序图。避免巨型全景图、硬编码亮色、装饰动画与重复截图。

数据链路：中文 MDX Mermaid fence → 既有生成器保留代码 → server embed → DocsContent 识别 mermaid → 复用已有 MermaidDiagram；文档站编译阶段将同一 fence 交给已有 Mermaid 渲染器。普通代码块保持既有 CodeBlock。不得引入 <Mermaid chart={...}/> 破坏封闭 JSX 转换合同。

应用内复用 editor 的 leaf 图示、sandbox 和 viewer，样式仅作用图示，不给文档根添加编辑器容器。公共站保持独立组件，不引入业务 views 依赖。具体 public fence 接法由实现子代理核对 Fumadocs 编译结果后选择最小路径。

图源使用 accTitle/accDescr（支持的类型），正文说明不依赖图中颜色传达含义。样式及安全改动若触及共享 editor 组件，保持 API 向后兼容并跑对应回归。

验证在独立的浏览器/桌面测试窗口完成，不触碰用户正在看的账号和工作内容。完成后需要在当前本地预览环境看到新增图，按已有进程身份重启后端刷新嵌入包。
