// Los temas de DaisyUI (incl. 'nothing' y 'terminal') se compilan desde
// tailwind.config.js — ya no se inyectan en runtime desde el CDN.
// Función única y global para el modal
function toggleModal() {
    const modal = document.getElementById('modal');
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

// Los modales se ABREN antes de pedir los datos, no despues.
//
// Sintoma: el endpoint respondia 200 con los 9 partidos, htmx lo recibia bien
// (afterRequest ok=true) y '#partidos-content' quedaba con 0 bytes. La razon: el
// onclick corria en el mismo click pero htmx resolvia el target cuando el modal
// todavia estaba cerrado, y un contenedor oculto mide 0x0 -> el swap no tenia donde
// escribir. En htmx 1.9.10 eso no tira error: falla en silencio y el modal queda
// mostrando el placeholder ("Buscando partidos...") para siempre.
//
// Con el modal ya visible el target tiene tamano real y el swap funciona. El
// hx-trigger de los botones lleva "click delay:10ms" para que el onclick abra el
// modal en el mismo evento y la peticion salga despues.
function abrirModalPartidos() {
    togglePartidosModal();
}
function abrirModalF1() {
    toggleF1Modal();
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
    if (form && (form.getAttribute('action') === '/settings' || form.closest('form[action="/settings"]'))) {
        const themeSelect = form.querySelector('select[name="theme"]');
        if (themeSelect && themeSelect.value) {
            document.documentElement.setAttribute('data-theme', themeSelect.value);
        }
    }
});
