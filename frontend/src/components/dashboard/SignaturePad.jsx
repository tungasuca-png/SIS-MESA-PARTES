import { useEffect, useRef, useState } from "react";
import "./signaturePad.css";

// Área de firma capturada a mano (canvas nativo, sin librerías). NO es una
// firma digital certificada — es una captura gráfica, tal como pide la
// skill de FUT Digital ("no inventar validez legal").
function SignaturePad({ onChange }) {
    const canvasRef = useRef(null);
    const drawingRef = useRef(false);
    const hasDrawnRef = useRef(false);
    const [hasDrawing, setHasDrawing] = useState(false);

    useEffect(() => {
        const canvas = canvasRef.current;
        const ctx = canvas.getContext("2d");
        ctx.lineWidth = 2;
        ctx.lineCap = "round";
        ctx.strokeStyle = "#1B2559";
    }, []);

    const pointFromEvent = (canvas, event) => {
        const rect = canvas.getBoundingClientRect();
        const point = event.touches ? event.touches[0] : event;
        return {
            x: ((point.clientX - rect.left) / rect.width) * canvas.width,
            y: ((point.clientY - rect.top) / rect.height) * canvas.height,
        };
    };

    const startDraw = (event) => {
        event.preventDefault();
        drawingRef.current = true;
        const canvas = canvasRef.current;
        const ctx = canvas.getContext("2d");
        const { x, y } = pointFromEvent(canvas, event);
        ctx.beginPath();
        ctx.moveTo(x, y);
    };

    const draw = (event) => {
        if (!drawingRef.current) return;
        event.preventDefault();
        const canvas = canvasRef.current;
        const ctx = canvas.getContext("2d");
        const { x, y } = pointFromEvent(canvas, event);
        ctx.lineTo(x, y);
        ctx.stroke();
        if (!hasDrawnRef.current) {
            hasDrawnRef.current = true;
            setHasDrawing(true);
        }
    };

    const endDraw = () => {
        if (!drawingRef.current) return;
        drawingRef.current = false;
        if (hasDrawnRef.current) {
            onChange?.(canvasRef.current.toDataURL("image/png"));
        }
    };

    const handleClear = () => {
        const canvas = canvasRef.current;
        const ctx = canvas.getContext("2d");
        ctx.clearRect(0, 0, canvas.width, canvas.height);
        hasDrawnRef.current = false;
        setHasDrawing(false);
        onChange?.(null);
    };

    return (
        <div className="dp-signature-pad">
            <canvas
                ref={canvasRef}
                width={500}
                height={160}
                className="dp-signature-canvas"
                onMouseDown={startDraw}
                onMouseMove={draw}
                onMouseUp={endDraw}
                onMouseLeave={endDraw}
                onTouchStart={startDraw}
                onTouchMove={draw}
                onTouchEnd={endDraw}
            />
            <div className="dp-signature-actions">
                <span className="dp-signature-hint">
                    {hasDrawing ? "Firma capturada" : "Dibuje su firma en el recuadro"}
                </span>
                <button type="button" className="dp-btn-secondary" onClick={handleClear}>
                    Limpiar
                </button>
            </div>
        </div>
    );
}

export default SignaturePad;
