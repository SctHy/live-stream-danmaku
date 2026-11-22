import { useState } from "react";
import * as Popover from "@radix-ui/react-popover";
import { settings } from "./danmaku";
import { Slider } from "./ui/Slider";

const fields = [
  ["density", "同屏上限", 0, 160, "条"],
  ["area", "显示区域", 10, 100, "%"],
  ["opacity", "透明度", 10, 100, "%"],
  ["size", "字号", 14, 42, "px"],
  ["speed", "速度", 60, 320, "px/s"],
] as const;

export function Settings({ resetOverlay }: { resetOverlay: () => void }) {
  const [current, setCurrent] = useState({ ...settings });
  function change<K extends keyof typeof settings>(
    key: K,
    value: (typeof settings)[K],
  ) {
    settings[key] = value;
    if (key !== "opacity" && key !== "speed") resetOverlay();
    setCurrent({ ...settings });
  }
  return (
    <Popover.Root>
      <Popover.Trigger>弹幕设置</Popover.Trigger>
      <Popover.Portal>
        <Popover.Content
          aria-label="弹幕设置"
          className="fields"
          sideOffset={8}
          collisionPadding={12}
        >
          {fields.map(([key, label, min, max, unit]) => (
            <div key={key}>
              <span>
                {label}{" "}
                <output>
                  {current[key]} {unit}
                </output>
              </span>
              <Slider
                label={label}
                value={current[key]}
                min={min}
                max={max}
                onChange={(n) => change(key, n)}
              />
            </div>
          ))}
          <div className="swatches">
            {["#ffffff", "#86dfbc", "#ffc990", "#b6bafa"].map((color) => (
              <button
                key={color}
                aria-label={"颜色 " + color}
                aria-pressed={current.color === color}
                style={{ background: color }}
                onClick={() => change("color", color)}
              />
            ))}
          </div>
          <label>
            文字颜色
            <input
              type="color"
              value={current.color}
              onChange={(e) => change("color", e.target.value)}
            />
          </label>
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  );
}
