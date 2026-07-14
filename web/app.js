let expenses = [];
let chartInstance = null;

// DOM Elements
const loginModal = document.getElementById('loginModal');
const appUI = document.getElementById('appUI');
const passcodeInput = document.getElementById('passcodeInput');
const loginBtn = document.getElementById('loginBtn');
const loginError = document.getElementById('loginError');

// Tab Switching (Mobile)
function switchTab(tabId) {
    document.querySelectorAll('.dashboard-panel, .chat-panel').forEach(el => el.classList.remove('active-tab'));
    document.querySelectorAll('.tab-btn').forEach(el => el.classList.remove('active'));
    
    document.getElementById(tabId + 'Tab').classList.add('active-tab');
    event.target.classList.add('active');
}

// Authentication
async function checkLogin() {
    const pwd = localStorage.getItem('web_password') || '';
    
    try {
        const res = await fetch('/api/login', {
            headers: { 'Authorization': 'Bearer ' + pwd }
        });
        const data = await res.json();
        
        if (data.success) {
            loginModal.style.display = 'none';
            appUI.style.display = 'flex';
            fetchData();
        } else {
            loginModal.style.display = 'flex';
            appUI.style.display = 'none';
        }
    } catch (e) {
        // Trừ khi network error, mặc định cho hiện Modal
        loginModal.style.display = 'flex';
    }
}

loginBtn.addEventListener('click', () => {
    localStorage.setItem('web_password', passcodeInput.value);
    checkLogin().then(() => {
        if (loginModal.style.display !== 'none') {
            loginError.style.display = 'block';
        } else {
            loginError.style.display = 'none';
        }
    });
});

passcodeInput.addEventListener('keypress', (e) => {
    if (e.key === 'Enter') loginBtn.click();
});

// Fetch Data
const fetchData = async () => {
    const pwd = localStorage.getItem('web_password') || '';
    try {
        const response = await fetch('/api/expenses', {
            headers: { 'Authorization': 'Bearer ' + pwd }
        });
        if (response.status === 401) {
            localStorage.removeItem('web_password');
            checkLogin();
            return;
        }
        const data = await response.json();
        expenses = data || [];
        updateDashboard();
    } catch (error) {
        console.error('Lỗi fetch data:', error);
    }
};

// Update Dashboard (Chart & List)
const updateDashboard = () => {
    let totalIncome = 0;
    let totalExpense = 0;
    const categoryTotals = {};

    expenses.forEach(exp => {
        if (exp.type === 'Thu' || exp.type === 'thu') {
            totalIncome += exp.amount;
        } else {
            totalExpense += exp.amount;
            categoryTotals[exp.category] = (categoryTotals[exp.category] || 0) + exp.amount;
        }
    });

    const formatVND = (num) => new Intl.NumberFormat('vi-VN', { style: 'currency', currency: 'VND' }).format(num);

    document.getElementById('totalIncome').innerText = formatVND(totalIncome);
    document.getElementById('totalExpense').innerText = formatVND(totalExpense);
    document.getElementById('netBalance').innerText = formatVND(totalIncome - totalExpense);
    document.getElementById('txCount').innerText = expenses.length;

    // Render Transactions
    const listEl = document.getElementById('transactionsList');
    listEl.innerHTML = '';
    const recent = [...expenses].reverse().slice(0, 10);
    
    if (recent.length === 0) {
        listEl.innerHTML = '<p style="text-align:center; color:var(--text-secondary)">Chưa có giao dịch nào.</p>';
    }

    recent.forEach(exp => {
        const item = document.createElement('div');
        item.className = 'transaction-item glass-card';
        const isIncome = (exp.type === 'Thu' || exp.type === 'thu');
        const colorClass = isIncome ? 'positive' : 'negative';
        const sign = isIncome ? '+' : '-';
        
        item.innerHTML = `
            <div>
                <strong style="display:block">${exp.description}</strong>
                <small style="color:var(--text-secondary)">${exp.date} • ${exp.category}</small>
            </div>
            <div class="${colorClass}" style="font-weight:700">
                ${sign}${formatVND(exp.amount)}
            </div>
        `;
        listEl.appendChild(item);
    });

    // Render Chart
    const ctx = document.getElementById('expenseChart').getContext('2d');
    if (chartInstance) chartInstance.destroy();

    const labels = Object.keys(categoryTotals);
    const data = Object.values(categoryTotals);
    const bgColors = ['#ef4444', '#f59e0b', '#3b82f6', '#10b981', '#8b5cf6', '#ec4899', '#6366f1'];

    if (labels.length === 0) {
        labels.push('Chưa có dữ liệu');
        data.push(1);
        bgColors[0] = '#d1d5db';
    }

    chartInstance = new Chart(ctx, {
        type: 'doughnut',
        data: {
            labels: labels,
            datasets: [{
                data: data,
                backgroundColor: bgColors,
                borderWidth: 0
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            plugins: {
                legend: { position: 'right', labels: { color: getComputedStyle(document.body).getPropertyValue('--text-primary') } }
            }
        }
    });
};

// Chat UI Logic
const chatHistory = document.getElementById('chatHistory');
const chatForm = document.getElementById('chatForm');
const chatInput = document.getElementById('chatInput');
const imageUpload = document.getElementById('imageUpload');
const imagePreview = document.getElementById('imagePreview');
const previewImg = document.getElementById('previewImg');

let selectedFile = null;

function appendMessage(text, isUser = false, imgUrl = null) {
    const msgDiv = document.createElement('div');
    msgDiv.className = `message ${isUser ? 'user-message' : 'ai-message'}`;
    
    if (imgUrl) {
        const img = document.createElement('img');
        img.src = imgUrl;
        msgDiv.appendChild(img);
    }
    
    const p = document.createElement('p');
    p.innerHTML = text.replace(/\n/g, '<br>');
    msgDiv.appendChild(p);
    
    chatHistory.appendChild(msgDiv);
    chatHistory.scrollTop = chatHistory.scrollHeight;
}

function previewImage(event) {
    const file = event.target.files[0];
    if (file) {
        selectedFile = file;
        const reader = new FileReader();
        reader.onload = function(e) {
            previewImg.src = e.target.result;
            imagePreview.style.display = 'inline-block';
        }
        reader.readAsDataURL(file);
    }
}

function removeImage() {
    selectedFile = null;
    imageUpload.value = '';
    imagePreview.style.display = 'none';
}

chatForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    const text = chatInput.value.trim();
    if (!text && !selectedFile) return;

    let imgDataUrl = null;
    if (selectedFile) imgDataUrl = previewImg.src;

    appendMessage(text || "📷 Đã gửi một ảnh", true, imgDataUrl);
    
    const formData = new FormData();
    formData.append('text', text);
    if (selectedFile) formData.append('image', selectedFile);

    chatInput.value = '';
    removeImage();
    
    appendMessage("⏳ Đang suy nghĩ...", false);
    const typingMsg = chatHistory.lastElementChild;

    try {
        const pwd = localStorage.getItem('web_password') || '';
        const res = await fetch('/api/chat', {
            method: 'POST',
            headers: { 'Authorization': 'Bearer ' + pwd },
            body: formData
        });
        
        if (res.status === 401) {
            typingMsg.remove();
            appendMessage("❌ Phiên đăng nhập hết hạn, vui lòng tải lại trang.", false);
            return;
        }

        const data = await res.json();
        typingMsg.remove();
        
        if (data.success) {
            appendMessage(data.reply, false);
            if (data.expenses) {
                expenses = [...expenses, ...data.expenses];
                updateDashboard();
            }
        } else {
            appendMessage(data.reply || "❌ Có lỗi xảy ra", false);
        }
    } catch (err) {
        typingMsg.remove();
        appendMessage("❌ Lỗi mạng: " + err.message, false);
    }
});

// Theme Toggle
const themeToggle = document.getElementById('themeToggle');
const savedTheme = localStorage.getItem('theme') || 'light';
if (savedTheme === 'dark') {
    document.body.setAttribute('data-theme', 'dark');
    themeToggle.innerText = '☀️';
}

themeToggle.addEventListener('click', () => {
    const isDark = document.body.getAttribute('data-theme') === 'dark';
    if (isDark) {
        document.body.removeAttribute('data-theme');
        localStorage.setItem('theme', 'light');
        themeToggle.innerText = '🌙';
    } else {
        document.body.setAttribute('data-theme', 'dark');
        localStorage.setItem('theme', 'dark');
        themeToggle.innerText = '☀️';
    }
    if (chartInstance) updateDashboard();
});

// Init
checkLogin();
