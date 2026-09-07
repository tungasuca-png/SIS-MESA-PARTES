import { useState } from "react";
import Sidebar from "./Sidebar";
import Header from "./Header";
import "./dashboard-theme.css";
import "./DashboardLayout.css";

function DashboardLayout({ title, children }) {
    const [sidebarOpen, setSidebarOpen] = useState(false);

    return (
        <div className="dashboard-shell">
            <Sidebar open={sidebarOpen} onClose={() => setSidebarOpen(false)} />

            {sidebarOpen && (
                <button
                    type="button"
                    className="dp-overlay"
                    aria-label="Cerrar menú"
                    onClick={() => setSidebarOpen(false)}
                />
            )}

            <div className="dp-content">
                <Header title={title} onToggleSidebar={() => setSidebarOpen((open) => !open)} />
                <main className="dp-main">{children}</main>
            </div>
        </div>
    );
}

export default DashboardLayout;
