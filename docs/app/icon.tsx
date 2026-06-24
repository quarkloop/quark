import { ImageResponse } from "next/og";

export const runtime = "edge";

export const size = { width: 32, height: 32 };
export const contentType = "image/png";

export default function Icon() {
  return new ImageResponse(
    (
      <div
        style={{
          width: "100%",
          height: "100%",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          backgroundColor: "#e87015",
          borderRadius: "8px",
          fontSize: "20px",
          fontWeight: 700,
          color: "#fdf8f0",
          fontFamily: "sans-serif",
        }}
      >
        Q
      </div>
    ),
    { ...size }
  );
}
