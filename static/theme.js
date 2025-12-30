let isDark = false;

function toggleTheme() {
    isDark = !isDark;
    document.body.className = isDark ? 'dark' : 'light';
    document.getElementById('moonIcon').classList.toggle('hidden');
    document.getElementById('sunIcon').classList.toggle('hidden');
    localStorage.setItem('theme', isDark ? 'dark' : 'light');
}

// Load saved theme
document.addEventListener('DOMContentLoaded', () => {
    const savedTheme = localStorage.getItem('theme');
    if (savedTheme === 'dark') {
        toggleTheme();
    }
});
