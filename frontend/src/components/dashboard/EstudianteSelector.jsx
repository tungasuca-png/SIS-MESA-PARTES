import { useEffect, useRef, useState } from "react";
import { buscarEstudiantesPsicologia } from "../../services/psicologiaService";
import { friendlyErrorMessage } from "../../utils/apiErrors";
import Icon from "./Icon";
import "./EstudianteSelector.css";

const MIN_QUERY_LENGTH = 2;
const PAGE_SIZE = 10;
const DEBOUNCE_MS = 350;

// EstudianteSelector: buscador de estudiantes por nombres, apellidos o DNI
// completo para "Nuevo expediente psicológico" y "Buscar alumno". Consume
// POST /api/psicologia/estudiantes/buscar: Psicología exige los permisos
// psicologia.expedientes.* del usuario y consulta Estudiantes con su
// credencial de servicio (el usuario NO necesita estudiantes.view). Solo
// recibe y muestra nombre, grado y sección: nunca el DNI ni códigos, aunque
// se haya buscado por DNI. El backend sigue validando OpenExpediente.
//
// value: el estudiante YA seleccionado ({id, nombreCompleto, ...}) o null.
// onChange(estudianteBasico | null): se llama al seleccionar o al limpiar.
function EstudianteSelector({ id, value, onChange, disabled }) {
    const [query, setQuery] = useState("");
    const [status, setStatus] = useState("idle"); // idle | loading | results | empty | error
    const [results, setResults] = useState([]);
    const [total, setTotal] = useState(0);
    const [page, setPage] = useState(1);
    const [errorMessage, setErrorMessage] = useState("");
    const debounceRef = useRef(null);
    const requestRef = useRef(0);

    const runSearch = (q, pageToLoad, append) => {
        const requestId = ++requestRef.current;
        if (!append) setStatus("loading");
        buscarEstudiantesPsicologia({ q, page: pageToLoad, pageSize: PAGE_SIZE })
            .then((data) => {
                if (requestRef.current !== requestId) return; // respuesta obsoleta
                const items = data.estudiantes || [];
                setTotal(data.total || 0);
                setPage(pageToLoad);
                if (append) {
                    // "Ver más": conserva lo que ya había, aunque la página
                    // nueva venga vacía (no debería pasar, pero no rompe la vista).
                    setResults((current) => [...current, ...items]);
                    setStatus("results");
                } else {
                    setResults(items);
                    setStatus(items.length > 0 ? "results" : "empty");
                }
            })
            .catch((err) => {
                if (requestRef.current !== requestId) return;
                setErrorMessage(friendlyErrorMessage(err, "No se pudo consultar la lista de estudiantes."));
                setStatus("error");
            });
    };

    // Solo limpia el debounce pendiente al desmontar — no dispara la
    // búsqueda por sí solo, así que no hay setState en el cuerpo de un
    // efecto: eso pasa en el manejador de eventos de abajo (handleQueryChange).
    useEffect(() => () => clearTimeout(debounceRef.current), []);

    const handleQueryChange = (event) => {
        const nextQuery = event.target.value;
        setQuery(nextQuery);
        clearTimeout(debounceRef.current);

        const trimmed = nextQuery.trim();
        if (trimmed.length < MIN_QUERY_LENGTH) {
            requestRef.current++; // invalida cualquier búsqueda en vuelo
            setStatus("idle");
            setResults([]);
            setTotal(0);
            return;
        }

        debounceRef.current = setTimeout(() => runSearch(trimmed, 1, false), DEBOUNCE_MS);
    };

    const handleVerMas = () => {
        const trimmed = query.trim();
        if (trimmed.length < MIN_QUERY_LENGTH) return;
        runSearch(trimmed, page + 1, true);
    };

    const handleSelect = (estudiante) => {
        setQuery("");
        setResults([]);
        setTotal(0);
        setStatus("idle");
        onChange(estudiante);
    };

    const handleChangeStudent = () => {
        onChange(null);
        setQuery("");
        setStatus("idle");
    };

    if (value) {
        return (
            <div className="dp-form-group">
                <label>Estudiante *</label>
                <div className="dp-estudiante-selected">
                    <div>
                        <p className="dp-estudiante-selected-name">{value.nombre_completo}</p>
                        <p className="dp-estudiante-selected-meta">{gradoSeccion(value)}</p>
                    </div>
                    {!disabled && (
                        <button type="button" className="dp-btn-secondary" onClick={handleChangeStudent}>
                            Cambiar estudiante
                        </button>
                    )}
                </div>
            </div>
        );
    }

    return (
        <div className="dp-form-group">
            <label htmlFor={id}>Estudiante *</label>
            <div className="dp-estudiante-search">
                <Icon name="search" size={16} className="dp-estudiante-search-icon" />
                <input
                    id={id}
                    type="text"
                    value={query}
                    onChange={handleQueryChange}
                    disabled={disabled}
                    placeholder="Buscar por nombres, apellidos o DNI completo..."
                    autoComplete="off"
                />
            </div>

            {status === "idle" && query.trim().length > 0 && (
                <p className="dp-form-hint">Escribe al menos {MIN_QUERY_LENGTH} caracteres para buscar.</p>
            )}
            {status === "loading" && <p className="dp-form-hint">Buscando estudiantes...</p>}
            {status === "error" && <p className="dp-form-error">{errorMessage}</p>}
            {status === "empty" && <p className="dp-form-hint">No se encontraron estudiantes.</p>}

            {status === "results" && results.length > 0 && (
                <div className="dp-estudiante-results">
                    {results.map((item) => (
                        <button
                            type="button"
                            key={item.id}
                            className="dp-estudiante-result-item"
                            onClick={() => handleSelect(item)}
                        >
                            <span className="dp-estudiante-result-name">{item.nombre_completo}</span>
                            <span className="dp-estudiante-result-meta">{gradoSeccion(item)}</span>
                        </button>
                    ))}
                    {results.length < total && (
                        <button type="button" className="dp-btn-secondary dp-estudiante-ver-mas" onClick={handleVerMas}>
                            Ver más resultados
                        </button>
                    )}
                </div>
            )}
        </div>
    );
}

// "3.º B": lo único que se muestra además del nombre (distingue homónimos).
function gradoSeccion(estudiante) {
    return [estudiante.grado && `${estudiante.grado}.º`, estudiante.seccion].filter(Boolean).join(" ");
}

export default EstudianteSelector;
