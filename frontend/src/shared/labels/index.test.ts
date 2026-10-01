import { describe, expect, it } from "vitest";
import {
  CATEGORIES,
  CATEGORY_LABEL_KEY,
  DIFFICULTIES,
  DIFFICULTY_HOME_TEXT_CLASS,
  DIFFICULTY_LABEL_KEY,
  DIFFICULTY_TEXT_CLASS,
  DIFFICULTY_TONE,
  DIMENSION_ORDER,
  antiPatternLabel,
  categoryLabel,
  difficultyLabel,
  dimensionLabel,
} from "@/shared/labels";

describe("labels", () => {
  it("has a key and colour for every category / difficulty", () => {
    for (const c of CATEGORIES) expect(CATEGORY_LABEL_KEY[c]).toBeTruthy();
    for (const d of DIFFICULTIES) {
      expect(DIFFICULTY_LABEL_KEY[d]).toBeTruthy();
      expect(DIFFICULTY_TEXT_CLASS[d]).toMatch(/^text-/);
      expect(DIFFICULTY_HOME_TEXT_CLASS[d]).toMatch(/^text-home-/);
      expect(DIFFICULTY_TONE[d]).toBeTruthy();
    }
  });

  it("renders English labels", () => {
    expect(difficultyLabel("easy")).toBe("Easy");
    expect(categoryLabel("feature_build")).not.toBe("feature_build");
    expect(antiPatternLabel("hands_off")).not.toBe("hands_off");
  });

  it("maps both spellings of the verification dimension to one label", () => {
    expect(dimensionLabel("verification")).toBe(dimensionLabel("verification_quality"));
    for (const d of DIMENSION_ORDER) expect(dimensionLabel(d)).not.toBe(d);
  });

  it("falls back to the raw value for unknown enums", () => {
    expect(categoryLabel("brand_new")).toBe("brand_new");
    expect(dimensionLabel("vibes")).toBe("vibes");
    expect(antiPatternLabel("x")).toBe("x");
  });
});
