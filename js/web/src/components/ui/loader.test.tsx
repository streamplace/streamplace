import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { Loader } from "./loader";

describe("Loader", () => {
  it("renders an accessible twelve-bar loading indicator", () => {
    const container = document.createElement("div");
    container.innerHTML = renderToStaticMarkup(
      <Loader label="Buffering" className="text-white" />,
    );

    const loader = container.querySelector('[role="status"]');
    const items = container.querySelectorAll(".ui-loader__item");

    expect(loader?.getAttribute("aria-label")).toBe("Buffering");
    expect(loader?.classList.contains("text-white")).toBe(true);
    expect((loader as HTMLElement).style.filter).toBe("");
    expect(items).toHaveLength(12);
    expect(
      (items[0] as HTMLElement).style.getPropertyValue("--loader-angle"),
    ).toBe("30deg");
    expect(
      (items[11] as HTMLElement).style.getPropertyValue("--loader-angle"),
    ).toBe("360deg");
    expect(
      (items[11] as HTMLElement).style.getPropertyValue("--loader-delay"),
    ).toBe("-0.6111111111s");
  });
});
