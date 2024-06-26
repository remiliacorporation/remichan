import { page } from '../state'

export default function createMobileBoardSelectEvent() {
    const bannerMobileNav = document.querySelector('#board-select-mobile') as HTMLSelectElement;
    bannerMobileNav.addEventListener('change', (event) => {
        var selectElement = event.target as HTMLOptionElement;
        var value = selectElement.value;
        window.location.href = `/${value}`;
    });

    bannerMobileNav.value = page.board;
}

