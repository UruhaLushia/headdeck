import { LineChart } from "echarts/charts";
import { AriaComponent, GridComponent, TooltipComponent } from "echarts/components";
import { init, use as registerCharts } from "echarts/core";
import type { EChartsType } from "echarts/core";
import { SVGRenderer } from "echarts/renderers";
import { useEffect, useEffectEvent, useId, useRef } from "react";
import { Panel } from "../../components/Page";
import { t, useLocale } from "../../lib/i18n";

registerCharts([LineChart, GridComponent, TooltipComponent, AriaComponent, SVGRenderer]);

export interface MetricSample {
  time: string;
  values: Record<string, number>;
}

function formatValue(value: number, unit: string) {
  if (unit === "bytes") return `${(value / 1048576).toFixed(1)} MiB`;
  if (unit === "seconds") return `${value.toFixed(1)} s`;
  return value.toLocaleString(undefined, { maximumFractionDigits: 1 });
}

export function MetricChart({
  name,
  label,
  unit,
  samples,
}: {
  name: string;
  label: string;
  unit: string;
  samples: MetricSample[];
}) {
  const id = useId();
  const locale = useLocale();
  const container = useRef<HTMLDivElement>(null);
  const chart = useRef<EChartsType | null>(null);
  const latest = samples.at(-1);
  const value = latest?.values[name];
  const render = useEffectEvent(() => {
    const dark = document.documentElement.dataset.theme === "dark";
    const muted = dark ? "#a1a1aa" : "#71717a";
    const border = dark ? "#3f3f46" : "#e4e4e7";
    const end = latest ? new Date(latest.time).getTime() : undefined;
    chart.current?.setOption(
      {
        animation: false,
        darkMode: dark,
        aria: { enabled: true, label: { description: t("{0} trend", t(label)) } },
        color: ["#0088ff"],
        textStyle: { fontFamily: getComputedStyle(container.current!).fontFamily },
        grid: { left: 4, right: 12, top: 16, bottom: 4, containLabel: true },
        tooltip: {
          trigger: "axis",
          confine: true,
          renderMode: "richText",
          backgroundColor: dark ? "#27272a" : "#ffffff",
          borderColor: border,
          textStyle: { color: muted, fontSize: 12 },
          valueFormatter: (raw: number | null) => (raw == null ? "—" : formatValue(raw, unit)),
        },
        xAxis: {
          type: "time",
          splitNumber: 3,
          min: samples.length === 1 && end !== undefined ? end - 60000 : undefined,
          max: end,
          axisLabel: {
            color: muted,
            fontSize: 11,
            hideOverlap: true,
            formatter: (time: number) =>
              new Date(time).toLocaleTimeString(locale, {
                hour: "2-digit",
                minute: "2-digit",
                second: "2-digit",
              }),
          },
          axisLine: { lineStyle: { color: border } },
          axisTick: { show: false },
        },
        yAxis: {
          type: "value",
          min: 0,
          splitNumber: 3,
          minInterval: unit === "count" ? 1 : undefined,
          axisLabel: {
            color: muted,
            fontSize: 11,
            formatter: (tick: number) => formatValue(tick, unit),
          },
          splitLine: { lineStyle: { color: border, type: "dashed" } },
        },
        series: [
          {
            name: t(label),
            type: "line",
            smooth: false,
            connectNulls: false,
            showSymbol: samples.length < 3,
            symbolSize: 6,
            lineStyle: { width: 2 },
            areaStyle: { opacity: 0.08 },
            data: samples.map((sample) => [
              new Date(sample.time).getTime(),
              sample.values[name] ?? null,
            ]),
          },
        ],
      },
      { notMerge: true },
    );
  });

  useEffect(() => {
    chart.current = init(container.current!, null, { renderer: "svg" });
    const resize = new ResizeObserver(() => chart.current?.resize());
    resize.observe(container.current!);
    const theme = new MutationObserver(() => render());
    theme.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["data-theme"],
    });
    return () => {
      resize.disconnect();
      theme.disconnect();
      chart.current?.dispose();
      chart.current = null;
    };
  }, []);

  useEffect(() => {
    render();
  }, [samples, name, label, unit, locale]);

  return (
    <Panel>
      <figure aria-labelledby={id} className="min-w-0">
        <figcaption id={id} className="text-sm text-muted">
          {t(label)}
        </figcaption>
        <div className="mt-2 flex min-h-8 flex-wrap items-baseline justify-between gap-x-2 gap-y-1">
          <p className="text-2xl font-semibold tabular-nums">
            {value === undefined ? "—" : formatValue(value, unit)}
          </p>
          {latest && (
            <p className="text-xs text-muted tabular-nums">
              {new Date(latest.time).toLocaleTimeString(locale)}
            </p>
          )}
        </div>
        <div className="relative mt-2">
          <div ref={container} className="h-44 w-full" aria-label={t("{0} trend", t(label))} />
          {value === undefined && (
            <p className="pointer-events-none absolute inset-0 grid place-items-center bg-surface text-xs text-muted">
              {t("No samples yet")}
            </p>
          )}
        </div>
      </figure>
    </Panel>
  );
}
