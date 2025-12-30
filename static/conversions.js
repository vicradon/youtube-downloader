let autoRefreshInterval = null;

function startAutoRefresh() {
    stopAutoRefresh();
    autoRefreshInterval = setInterval(() => {
        renderConversions();
    }, 2000);
}

function stopAutoRefresh() {
    if (autoRefreshInterval) {
        clearInterval(autoRefreshInterval);
        autoRefreshInterval = null;
    }
}

function renderConversions() {
    fetch('/api/conversions')
        .then(response => response.json())
        .then(jobs => {
            const container = document.getElementById('conversionsList');

            if (jobs.length === 0) {
                container.innerHTML = '<p class="empty-state">No conversions yet</p>';
                return;
            }

            container.innerHTML = jobs.map(job => {
                const statusClass = `status-${job.status}`;
                const startTime = new Date(job.startTime).toLocaleString();

                let actions = '';
                if (job.status === 'completed' && job.filename) {
                    actions = `
                        <div class="conversion-actions">
                            <a href="/api/file/${job.filename}" class="download-btn">Download</a>
                            <button onclick="deleteConversion('${job.filename}')" class="delete-btn">Delete</button>
                        </div>
                    `;
                } else if (job.status === 'failed') {
                    let errorHTML = `<div style="font-size: 13px; opacity: 0.6; margin-bottom: 8px;">Error: ${job.error || 'Unknown error'}</div>`;
                    if (job.canRetry) {
                        errorHTML += `
                            <div class="conversion-actions">
                                <button onclick="retryConversion('${job.id}')" class="download-btn">Retry</button>
                            </div>
                        `;
                    }
                    actions = errorHTML;
                }

                return `
                    <div class="conversion-item">
                        <div class="conversion-header">
                            <div class="conversion-title">
                                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                                    <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/>
                                    <polyline points="7 10 12 15 17 10"/>
                                    <line x1="12" y1="15" x2="12" y2="3"/>
                                </svg>
                                ${job.format.toUpperCase()} Conversion
                            </div>
                            <span class="conversion-status ${statusClass}">${job.status}</span>
                        </div>
                        <div class="conversion-url">${job.url}</div>
                        <div class="conversion-time">Started: ${startTime}</div>
                        ${actions}
                    </div>
                `;
            }).join('');
        })
        .catch(error => {
            console.error('Error:', error);
        });
}

function deleteConversion(filename) {
    if (!confirm('Are you sure you want to delete this file?')) {
        return;
    }

    fetch(`/api/delete/${filename}`, {
        method: 'DELETE'
    })
    .then(response => response.json())
    .then(data => {
        renderConversions();
    })
    .catch(error => {
        console.error('Error:', error);
    });
}

function retryConversion(jobId) {
    fetch(`/api/retry/${jobId}`, {
        method: 'POST'
    })
    .then(response => response.json())
    .then(data => {
        renderConversions();
    })
    .catch(error => {
        console.error('Error:', error);
    });
}

// Start auto-refresh when page loads
document.addEventListener('DOMContentLoaded', () => {
    renderConversions();
    startAutoRefresh();
});

// Stop auto-refresh when page is hidden
document.addEventListener('visibilitychange', () => {
    if (document.hidden) {
        stopAutoRefresh();
    } else {
        startAutoRefresh();
    }
});
