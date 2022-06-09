"use strict";

function mobileNavToggle() {
    const menu = document.getElementById('mobile-menu')?.parentElement;
    menu?.classList.toggle('mobile-menu-visible');
}

function docsVersionToggle() {
    const menu = document.getElementById('dropdown-menu');
    menu?.classList.toggle('dropdown-menu-visible');
}

window.onclick = function (event) {
    const target = event.target;
    const menu = document.getElementById('dropdown-menu');

    if (!target?.classList.contains('dropdown-toggle')) {
        menu?.classList.remove('dropdown-menu-visible');
    }
}
