---
name: frontend
description: 前端开发规范。写任何 React / TS / Tailwind 代码前必读。
---

## 组件
- 全部函数组件 + Hooks，禁止 class 组件
- 一个文件一个组件，文件名 PascalCase，默认导出
- 组件超过 150 行就拆；超过 3 个 props 就用对象解构
- 展示组件不做数据请求，数据请求放 `hooks/` 里的自定义 Hook

## 类型
- 禁止 `any`，用 `unknown` + 类型守卫
- API 返回类型统一放 `src/api/types.ts`，与 docs/API.md 一一对应
- 所有 API 函数返回 `Promise<T>`，错误抛 `ApiError`

## 数据请求
- 一律用 TanStack Query，不写裸 useEffect + fetch
- queryKey 规范：`['documents', { type, page }]`、`['document', slug]`、`['notes', 'tree']`
- 列表页必须处理 loading / empty / error 三态，不许只写 happy path

## 样式
- Tailwind 原子类，不写 .css 文件（全局变量除外）
- 颜色、间距只用 Tailwind 预设 token，不写魔法值
- 响应式：移动端优先，断点用 sm/md/lg
- 深色模式用 `dark:` 前缀，跟随系统

## Markdown 渲染
- 统一封装成 `<Markdown content={md} />`，禁止在页面里各写一遍
- 代码块必须有语言标识才能高亮，无语言则降级为纯文本
- 禁止 `dangerouslySetInnerHTML`，除非经过 DOMPurify

## 可访问性
- 所有可点击元素用 `<button>` 或 `<a>`，不用 div + onClick
- 图片必须有 alt；表单必须有 label
- 键盘可操作，焦点可见

## 提交前自检
- [ ] `npm run build` 通过，无 TS 报错
- [ ] `npm run lint` 通过
- [ ] 页面在 375px 宽度下不横向滚动
- [ ] 三态齐全
