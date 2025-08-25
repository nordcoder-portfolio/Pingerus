import { StrictMode, useEffect, useState } from "react";
import { createRoot } from "react-dom/client";
import "./styles/index.css";
import { AppRouter } from "./app/router";

function ThemeBoot() {
    const [ready, setReady] = useState(false);
    useEffect(() => {
        const saved = localStorage.getItem("theme") || "dark";
        document.documentElement.classList.toggle("dark", saved === "dark");
        setReady(true);
    }, []);
    return ready ? <AppRouter /> : null;
}

createRoot(document.getElementById("root")!).render(
    <StrictMode>
        <ThemeBoot />
    </StrictMode>
);