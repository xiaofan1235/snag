// cx —— 极小 classnames 合并（不引三方依赖）。falsy 过滤，空格连接。
export function cx(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(" ");
}
