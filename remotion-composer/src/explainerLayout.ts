// Fit the authored 1080p canvas without changing its proportions or cropping it.
export function explainerLayout(viewportWidth: number, viewportHeight: number) {
  const width = 1920;
  const height = 1080;
  const scale = Math.min(viewportWidth / width, viewportHeight / height);
  return {
    width,
    height,
    scale,
    left: (viewportWidth - width * scale) / 2,
    top: (viewportHeight - height * scale) / 2,
  };
}
