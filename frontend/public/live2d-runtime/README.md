# Live2D Runtime

请将 `live2dcubismcore.min.js` 放到当前目录。

目标路径：

```text
frontend/public/live2d-runtime/live2dcubismcore.min.js
```

说明：

- 前端真实渲染使用 `pixi-live2d-display`，运行时依赖 Live2D Cubism Core。
- 当前代码会优先读取本地文件；本地缺失时才会尝试远程地址兜底。
- 生产环境建议始终提供本地文件，避免因外网不可用导致 Live2D 无法渲染。
