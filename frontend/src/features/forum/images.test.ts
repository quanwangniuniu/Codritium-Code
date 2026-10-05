import { describe, expect, it } from "vitest";
import { altFromFileName, isImageFile } from "@/features/forum/images";

describe("forum images", () => {
  it("derives alt text from file names", () => {
    expect(altFromFileName("my-diagram_v2.png")).toBe("my diagram v2");
    expect(altFromFileName("[evil](x).jpg")).toBe("evil(x)");
    expect(altFromFileName(".png")).toBe("image");
  });

  it("accepts only the uploadable image types", () => {
    const file = (type: string) => new File([""], "f", { type });
    expect(isImageFile(file("image/png"))).toBe(true);
    expect(isImageFile(file("image/gif"))).toBe(true);
    expect(isImageFile(file("image/svg+xml"))).toBe(false);
    expect(isImageFile(file("text/plain"))).toBe(false);
  });
});
