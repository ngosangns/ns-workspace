import type { JSX } from "solid-js";

type IconProps = {
  size?: number;
  weight?: "regular" | "bold" | "fill";
  class?: string;
  "aria-hidden"?: boolean | "true" | "false";
};

function svg(props: IconProps, paths: JSX.Element): JSX.Element {
  const size = props.size ?? 20;
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width={size}
      height={size}
      fill="none"
      viewBox="0 0 256 256"
      class={props.class}
      aria-hidden={props["aria-hidden"] ?? true}
      role="img"
    >
      <title>icon</title>
      {paths}
    </svg>
  );
}

const stroke = (weight?: string) => (weight === "bold" ? 20 : 16);

export function PhSquaresFour(props: IconProps) {
  return svg(
    props,
    <>
      <rect
        x="32"
        y="32"
        width="80"
        height="80"
        rx="8"
        stroke="currentColor"
        stroke-width={stroke(props.weight)}
        fill={props.weight === "fill" ? "currentColor" : "none"}
      />
      <rect
        x="144"
        y="32"
        width="80"
        height="80"
        rx="8"
        stroke="currentColor"
        stroke-width={stroke(props.weight)}
        fill={props.weight === "fill" ? "currentColor" : "none"}
      />
      <rect
        x="32"
        y="144"
        width="80"
        height="80"
        rx="8"
        stroke="currentColor"
        stroke-width={stroke(props.weight)}
        fill={props.weight === "fill" ? "currentColor" : "none"}
      />
      <rect
        x="144"
        y="144"
        width="80"
        height="80"
        rx="8"
        stroke="currentColor"
        stroke-width={stroke(props.weight)}
        fill={props.weight === "fill" ? "currentColor" : "none"}
      />
    </>,
  );
}

export function PhBrain(props: IconProps) {
  return svg(
    props,
    <path
      d="M208 76a44 44 0 0 0-68-36 44 44 0 0 0-68 36c0 40 24 56 32 88h72c8-32 32-48 32-88Z"
      stroke="currentColor"
      stroke-width={stroke(props.weight)}
      fill={props.weight === "fill" ? "currentColor" : "none"}
      stroke-linejoin="round"
    />,
  );
}

export function PhHardDrives(props: IconProps) {
  return svg(
    props,
    <>
      <rect
        x="40"
        y="48"
        width="176"
        height="64"
        rx="8"
        stroke="currentColor"
        stroke-width={stroke(props.weight)}
        fill={props.weight === "fill" ? "currentColor" : "none"}
      />
      <rect
        x="40"
        y="144"
        width="176"
        height="64"
        rx="8"
        stroke="currentColor"
        stroke-width={stroke(props.weight)}
        fill={props.weight === "fill" ? "currentColor" : "none"}
      />
      <circle cx="180" cy="80" r="8" fill={props.weight === "fill" ? "var(--color-app)" : "currentColor"} />
      <circle cx="180" cy="176" r="8" fill={props.weight === "fill" ? "var(--color-app)" : "currentColor"} />
    </>,
  );
}

export function PhCirclesFour(props: IconProps) {
  return svg(
    props,
    <>
      <circle
        cx="76"
        cy="76"
        r="28"
        stroke="currentColor"
        stroke-width={stroke(props.weight)}
        fill={props.weight === "fill" ? "currentColor" : "none"}
      />
      <circle
        cx="180"
        cy="76"
        r="28"
        stroke="currentColor"
        stroke-width={stroke(props.weight)}
        fill={props.weight === "fill" ? "currentColor" : "none"}
      />
      <circle
        cx="76"
        cy="180"
        r="28"
        stroke="currentColor"
        stroke-width={stroke(props.weight)}
        fill={props.weight === "fill" ? "currentColor" : "none"}
      />
      <circle
        cx="180"
        cy="180"
        r="28"
        stroke="currentColor"
        stroke-width={stroke(props.weight)}
        fill={props.weight === "fill" ? "currentColor" : "none"}
      />
    </>,
  );
}

export function PhPuzzlePiece(props: IconProps) {
  return svg(
    props,
    <path
      d="M64 104V64a16 16 0 0 1 16-16h40a20 20 0 1 1 0 40h16a20 20 0 1 0 0-40h40a16 16 0 0 1 16 16v40a20 20 0 1 0 40 0v16a20 20 0 1 1-40 0v40a16 16 0 0 1-16 16h-40a20 20 0 1 1 0-40h-16a20 20 0 1 0 0 40H80a16 16 0 0 1-16-16v-40a20 20 0 1 0-40 0v-16a20 20 0 1 1 40 0Z"
      stroke="currentColor"
      stroke-width={stroke(props.weight)}
      fill={props.weight === "fill" ? "currentColor" : "none"}
      stroke-linejoin="round"
    />,
  );
}

export function PhList(props: IconProps) {
  return svg(
    props,
    <>
      <line x1="40" y1="64" x2="216" y2="64" stroke="currentColor" stroke-width={stroke(props.weight)} stroke-linecap="round" />
      <line x1="40" y1="128" x2="216" y2="128" stroke="currentColor" stroke-width={stroke(props.weight)} stroke-linecap="round" />
      <line x1="40" y1="192" x2="216" y2="192" stroke="currentColor" stroke-width={stroke(props.weight)} stroke-linecap="round" />
    </>,
  );
}

export function PhFolder(props: IconProps) {
  return svg(
    props,
    <path
      d="M32 72V200a16 16 0 0 0 16 16h160a16 16 0 0 0 16-16V96a16 16 0 0 0-16-16h-72l-16-24H48A16 16 0 0 0 32 72Z"
      stroke="currentColor"
      stroke-width={stroke(props.weight)}
      fill={props.weight === "fill" ? "currentColor" : "none"}
      stroke-linejoin="round"
    />,
  );
}

export function PhFile(props: IconProps) {
  return svg(
    props,
    <path
      d="M200 224H56a8 8 0 0 1-8-8V40a8 8 0 0 1 8-8h96l56 56v128a8 8 0 0 1-8 8Z"
      stroke="currentColor"
      stroke-width={stroke(props.weight)}
      fill={props.weight === "fill" ? "currentColor" : "none"}
      stroke-linejoin="round"
    />,
  );
}

export function PhArrowCounterClockwise(props: IconProps) {
  return svg(
    props,
    <>
      <path
        d="M80 96H32V48"
        stroke="currentColor"
        stroke-width={stroke(props.weight)}
        stroke-linecap="round"
        stroke-linejoin="round"
        fill="none"
      />
      <path d="M32 96a96 96 0 1 1 28.3 68" stroke="currentColor" stroke-width={stroke(props.weight)} stroke-linecap="round" fill="none" />
    </>,
  );
}

export function PhFloppyDisk(props: IconProps) {
  return svg(
    props,
    <path
      d="M216 88v120a8 8 0 0 1-8 8H48a8 8 0 0 1-8-8V48a8 8 0 0 1 8-8h128l40 40Z"
      stroke="currentColor"
      stroke-width={stroke(props.weight)}
      fill={props.weight === "fill" ? "currentColor" : "none"}
      stroke-linejoin="round"
    />,
  );
}

export function PhArrowSquareOut(props: IconProps) {
  return svg(
    props,
    <>
      <path
        d="M168 40h48v48"
        stroke="currentColor"
        stroke-width={stroke(props.weight)}
        stroke-linecap="round"
        stroke-linejoin="round"
        fill="none"
      />
      <path d="M112 144 216 40" stroke="currentColor" stroke-width={stroke(props.weight)} stroke-linecap="round" fill="none" />
      <path
        d="M184 144v64a8 8 0 0 1-8 8H48a8 8 0 0 1-8-8V80a8 8 0 0 1 8-8h64"
        stroke="currentColor"
        stroke-width={stroke(props.weight)}
        stroke-linecap="round"
        fill="none"
      />
    </>,
  );
}

export function PhX(props: IconProps) {
  return svg(
    props,
    <>
      <line x1="200" y1="56" x2="56" y2="200" stroke="currentColor" stroke-width={stroke(props.weight)} stroke-linecap="round" />
      <line x1="200" y1="200" x2="56" y2="56" stroke="currentColor" stroke-width={stroke(props.weight)} stroke-linecap="round" />
    </>,
  );
}

export function PhCircleNotch(props: IconProps) {
  return svg(
    props,
    <path d="M168 40a96 96 0 1 1-40-8" stroke="currentColor" stroke-width={stroke(props.weight)} stroke-linecap="round" fill="none" />,
  );
}

export function PhMagnifyingGlass(props: IconProps) {
  return svg(
    props,
    <>
      <circle cx="112" cy="112" r="72" stroke="currentColor" stroke-width={stroke(props.weight)} fill="none" />
      <line x1="164.5" y1="164.5" x2="224" y2="224" stroke="currentColor" stroke-width={stroke(props.weight)} stroke-linecap="round" />
    </>,
  );
}

export function PhDownloadSimple(props: IconProps) {
  return svg(
    props,
    <>
      <path
        d="M80 168l48 48 48-48"
        stroke="currentColor"
        stroke-width={stroke(props.weight)}
        stroke-linecap="round"
        stroke-linejoin="round"
        fill="none"
      />
      <line x1="128" y1="40" x2="128" y2="216" stroke="currentColor" stroke-width={stroke(props.weight)} stroke-linecap="round" />
      <path d="M48 216h160" stroke="currentColor" stroke-width={stroke(props.weight)} stroke-linecap="round" />
    </>,
  );
}

export function PhPlus(props: IconProps) {
  return svg(
    props,
    <>
      <line x1="40" y1="128" x2="216" y2="128" stroke="currentColor" stroke-width={stroke(props.weight)} stroke-linecap="round" />
      <line x1="128" y1="40" x2="128" y2="216" stroke="currentColor" stroke-width={stroke(props.weight)} stroke-linecap="round" />
    </>,
  );
}

export function PhTrash(props: IconProps) {
  return svg(
    props,
    <>
      <path d="M216 56H40" stroke="currentColor" stroke-width={stroke(props.weight)} stroke-linecap="round" fill="none" />
      <path d="M104 104v64" stroke="currentColor" stroke-width={stroke(props.weight)} stroke-linecap="round" fill="none" />
      <path d="M152 104v64" stroke="currentColor" stroke-width={stroke(props.weight)} stroke-linecap="round" fill="none" />
      <path
        d="M200 56v152a8 8 0 0 1-8 8H64a8 8 0 0 1-8-8V56"
        stroke="currentColor"
        stroke-width={stroke(props.weight)}
        stroke-linecap="round"
        stroke-linejoin="round"
        fill="none"
      />
      <path
        d="M168 56V40a16 16 0 0 0-16-16h-48a16 16 0 0 0-16 16v16"
        stroke="currentColor"
        stroke-width={stroke(props.weight)}
        stroke-linecap="round"
        stroke-linejoin="round"
        fill="none"
      />
    </>,
  );
}
