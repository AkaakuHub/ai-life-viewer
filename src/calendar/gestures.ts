export function swipeDirection(x: number, y: number, distance: number) {
  if (Math.abs(x) < distance || Math.abs(x) < Math.abs(y) * 1.45) return 0;
  return x > 0 ? -1 : 1;
}

export function isInputTarget(target: EventTarget | null) {
  return (
    target instanceof HTMLElement &&
    (target.matches("input, textarea, select") || target.isContentEditable)
  );
}
