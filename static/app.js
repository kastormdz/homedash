window.tailwind = window.tailwind || {};
window.tailwind.config = {
    daisyui: {
        themes: [
            "dark", "dracula", "synthwave", "cyberpunk", "retro", "dim", "coffee", "sunset", "night",
            {
                "nothing": {
                    "primary": "#ffffff",
                    "base-100": "#000000",
                    "neutral": "#121212",
                    "accent": "#ff0000",
                    "--rounded-box": "2.5rem",
                    "--rounded-btn": "9999px"
                }
            }
        ]
    }
};

// Función única y global para el modal
function toggleModal() {
    const modal = document.getElementById('modal');
    if (modal) {
        modal.classList.toggle('active');
    }
}
function toggleFixtureModal() {
    const modal = document.getElementById('fixture-modal');
    if (modal) {
        modal.classList.toggle('active');
    }
}
function togglePartidosModal() {
    const modal = document.getElementById('partidos-modal');
    if (modal) {
        modal.classList.toggle('active');
    }
}
function toggleF1Modal() {
    const modal = document.getElementById('f1-modal');
    if (modal) {
        modal.classList.toggle('active');
    }
}

function updateClock() {
    const now = new Date();
    const options = { weekday: 'long', day: 'numeric', month: 'long' };
    const clockEl = document.getElementById('clock');
    const dateEl = document.getElementById('date');
    if (clockEl && dateEl) {
        clockEl.textContent = now.toLocaleTimeString('es-AR', { hour: '2-digit', minute: '2-digit', hour12: false });
        dateEl.textContent = now.toLocaleDateString('es-AR', options);
    }
}

function setupImageErrorHandlers() {
    document.querySelectorAll('img').forEach(img => {
        img.addEventListener('error', function() {
            if (this.classList.contains('f1-flag') || this.classList.contains('f1-poster') || this.classList.contains('ufc-headshot')) {
                this.style.display = 'none';
            } else if (this.classList.contains('team-crest')) {
                this.src = '/static/assets/crests/afa.png';
            }
        });
    });
}

// Inicialización
document.addEventListener('DOMContentLoaded', function() {
    lucide.createIcons();
    setupImageErrorHandlers();
    updateClock();
    setInterval(updateClock, 1000);
});

// Soporte para HTMX (Recarga parcial)
document.addEventListener('htmx:afterSwap', function(evt) { 
    lucide.createIcons(); 
    setupImageErrorHandlers();
});

// Actualizar tema dinámicamente al guardar configuración
document.addEventListener('htmx:afterRequest', function(evt) {
    const form = evt.detail.elt;
    if (form && form.getAttribute('action') === '/settings' || (form && form.closest('form[action="/settings"]'))) {
        const themeSelect = form.querySelector('select[name="theme"]');
        if (themeSelect && themeSelect.value) {
            document.documentElement.setAttribute('data-theme', themeSelect.value);
        }
    }
});
