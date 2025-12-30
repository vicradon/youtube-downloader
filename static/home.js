// Handle radio button changes
document.querySelectorAll('input[name="action"]').forEach(radio => {
    radio.addEventListener('change', function() {
        const formatGroup = document.getElementById('formatGroup');
        const directDownloadCard = document.getElementById('directDownloadCard');
        if (this.value === 'convert') {
            formatGroup.style.display = 'block';
            directDownloadCard.classList.add('hidden');
        } else {
            formatGroup.style.display = 'none';
        }
    });
});

function handleObtain() {
    const input = document.getElementById('videoUrl');
    const url = input.value.trim();
    const action = document.querySelector('input[name="action"]:checked').value;

    if (!url) {
        showError('Please enter a YouTube URL');
        return;
    }

    const btn = document.getElementById('obtainBtn');
    btn.disabled = true;
    btn.textContent = 'Processing...';
    hideError();

    const requestBody = {
        url: url,
        convert: action === 'convert'
    };

    if (action === 'convert') {
        const format = document.querySelector('input[name="format"]:checked').value;
        requestBody.format = format;
    }

    fetch('/api/download', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify(requestBody)
    })
    .then(response => response.json())
    .then(data => {
        input.value = '';
        btn.disabled = false;
        btn.textContent = 'obtain';

        if (data.status === 'ready') {
            document.getElementById('directDownloadLink').href = data.downloadUrl;
            document.getElementById('directDownloadCard').classList.remove('hidden');
        } else if (data.status === 'converting') {
            document.getElementById('directDownloadCard').classList.add('hidden');
            window.location.href = '/conversions';
        }
    })
    .catch(error => {
        console.error('Error:', error);
        showError('Failed to process video. Please try again.');
        btn.disabled = false;
        btn.textContent = 'obtain';
    });
}

function showError(message) {
    const errorDiv = document.getElementById('errorMessage');
    errorDiv.textContent = message;
    errorDiv.classList.remove('hidden');
}

function hideError() {
    document.getElementById('errorMessage').classList.add('hidden');
}

// Handle Enter key on input
document.getElementById('videoUrl').addEventListener('keypress', function(e) {
    if (e.key === 'Enter') {
        handleObtain();
    }
});
