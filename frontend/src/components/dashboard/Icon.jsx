const PATHS = {
    dashboard: (
        <>
            <rect x="3" y="3" width="7" height="9" rx="1.5" />
            <rect x="14" y="3" width="7" height="5" rx="1.5" />
            <rect x="14" y="12" width="7" height="9" rx="1.5" />
            <rect x="3" y="16" width="7" height="5" rx="1.5" />
        </>
    ),
    folder: <path d="M3 6.5A1.5 1.5 0 0 1 4.5 5h4l2 2.5h9A1.5 1.5 0 0 1 21 9v9.5A1.5 1.5 0 0 1 19.5 20h-15A1.5 1.5 0 0 1 3 18.5z" />,
    file: (
        <>
            <path d="M6 3.5A1.5 1.5 0 0 1 7.5 2H14l5 5v13.5A1.5 1.5 0 0 1 17.5 22h-10A1.5 1.5 0 0 1 6 20.5z" />
            <path d="M14 2v5h5" />
        </>
    ),
    share: (
        <>
            <circle cx="5.5" cy="12" r="2.5" />
            <circle cx="17.5" cy="5.5" r="2.5" />
            <circle cx="17.5" cy="18.5" r="2.5" />
            <path d="M7.7 10.7 15.3 6.8M7.7 13.3l7.6 3.9" />
        </>
    ),
    trending: (
        <>
            <path d="M3 17l6-6 4 4 8-8" />
            <path d="M15 6h6v6" />
        </>
    ),
    chart: (
        <>
            <path d="M4 20V10M11 20V4M18 20v-7" />
            <path d="M2 20h20" />
        </>
    ),
    users: (
        <>
            <circle cx="9" cy="8" r="3.2" />
            <path d="M3 20c0-3.3 2.7-6 6-6s6 2.7 6 6" />
            <circle cx="17.5" cy="9" r="2.4" />
            <path d="M15.5 14.3c2.6.4 4.5 2.6 4.5 5.7" />
        </>
    ),
    settings: (
        <>
            <circle cx="12" cy="12" r="3.2" />
            <path d="M19.4 13a7.6 7.6 0 0 0 0-2l2-1.5-2-3.4-2.4.7a7.7 7.7 0 0 0-1.7-1L14.8 3h-4l-.5 2.8a7.7 7.7 0 0 0-1.7 1l-2.4-.7-2 3.4L6.2 11a7.6 7.6 0 0 0 0 2l-2 1.5 2 3.4 2.4-.7c.5.4 1.1.8 1.7 1l.5 2.8h4l.5-2.8c.6-.2 1.2-.6 1.7-1l2.4.7 2-3.4z" />
        </>
    ),
    logout: (
        <>
            <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4" />
            <path d="M16 17l5-5-5-5" />
            <path d="M21 12H9" />
        </>
    ),
    plus: <path d="M12 5v14M5 12h14" />,
    bell: (
        <>
            <path d="M6 9a6 6 0 1 1 12 0c0 4 1.5 5.5 1.5 5.5H4.5S6 13 6 9Z" />
            <path d="M10 20a2 2 0 0 0 4 0" />
        </>
    ),
    chevronDown: <path d="M6 9l6 6 6-6" />,
    menu: <path d="M4 6h16M4 12h16M4 18h16" />,
    inbox: (
        <>
            <path d="M3 12h4.5l1.5 3h6l1.5-3H21" />
            <path d="M5.5 5h13L21 12v6.5A1.5 1.5 0 0 1 19.5 20h-15A1.5 1.5 0 0 1 3 18.5V12z" />
        </>
    ),
    clock: (
        <>
            <circle cx="12" cy="12" r="9" />
            <path d="M12 7v5l3.5 2" />
        </>
    ),
    loader: (
        <>
            <circle cx="12" cy="12" r="9" />
            <path d="M12 6a6 6 0 0 1 6 6" />
        </>
    ),
    check: (
        <>
            <circle cx="12" cy="12" r="9" />
            <path d="M8 12.5l2.5 2.5L16 9" />
        </>
    ),
    eye: (
        <>
            <path d="M2 12s3.8-7 10-7 10 7 10 7-3.8 7-10 7-10-7-10-7Z" />
            <circle cx="12" cy="12" r="2.6" />
        </>
    ),
    close: <path d="M6 6l12 12M18 6L6 18" />,
};

function Icon({ name, size = 18, className }) {
    const path = PATHS[name];
    if (!path) return null;

    return (
        <svg
            className={className}
            width={size}
            height={size}
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.8"
            strokeLinecap="round"
            strokeLinejoin="round"
            aria-hidden="true"
        >
            {path}
        </svg>
    );
}

export default Icon;
