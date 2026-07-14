let expenses = [];
let chartInstance = null;

// DOM Elements
const authScreen = document.getElementById('authScreen');
const appUI = document.getElementById('appUI');
const loginForm = document.getElementById('loginForm');
const registerForm = document.getElementById('registerForm');

function toggleAuth() {
    if (loginForm.style.display === 'none') {
        loginForm.style.display = 'block';
        registerForm.style.display = 'none';
    } else {
        loginForm.style.display = 'none';
        registerForm.style.display = 'block';
    }
}

async function handleLogin() {
    const user = document.getElementById('loginUsername').value;
    const pass = document.getElementById('loginPassword').value;
    const errEl = document.getElementById('loginError');
    
    try {
        const res = await fetch('/api/login', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({username: user, password: pass})
        });
        const data = await res.json();
        
        if (data.success) {
            localStorage.setItem('jwt_token', data.token);
            localStorage.setItem('username', data.user.username);
            localStorage.setItem('full_name', data.user.full_name);
            localStorage.setItem('role', data.user.role);
            
            document.getElementById('userNameDisplay').innerText = data.user.full_name;
            
            if (data.user.role === 'admin') {
                const navAdminBtn = document.getElementById('navAdminBtn');
                if (navAdminBtn) navAdminBtn.style.display = 'block';
            } else {
                const navAdminBtn = document.getElementById('navAdminBtn');
                if (navAdminBtn) navAdminBtn.style.display = 'none';
            }

            authScreen.style.display = 'none';
            appUI.style.display = 'flex';
            fetchData();
        } else {
            errEl.innerText = data.error || 'Đăng nhập thất bại';
            errEl.style.display = 'block';
        }
    } catch (e) {
        errEl.innerText = 'Lỗi kết nối máy chủ';
        errEl.style.display = 'block';
    }
}

async function handleRegister() {
    const fullname = document.getElementById('regFullName').value;
    const user = document.getElementById('regUsername').value;
    const pass = document.getElementById('regPassword').value;
    const errEl = document.getElementById('regError');
    const succEl = document.getElementById('regSuccess');
    
    try {
        const res = await fetch('/api/register', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({full_name: fullname, username: user, password: pass})
        });
        const data = await res.json();
        
        if (data.success) {
            errEl.style.display = 'none';
            succEl.style.display = 'block';
            setTimeout(() => {
                document.getElementById('loginUsername').value = user;
                document.getElementById('loginPassword').value = pass;
                toggleAuth();
                handleLogin();
            }, 1000);
        } else {
            errEl.innerText = data.error || 'Đăng ký thất bại';
            errEl.style.display = 'block';
        }
    } catch (e) {
        errEl.innerText = 'Lỗi kết nối máy chủ';
        errEl.style.display = 'block';
    }
}

function logout() {
    localStorage.removeItem('jwt_token');
    localStorage.removeItem('username');
    localStorage.removeItem('full_name');
    localStorage.removeItem('role');
    
    isAdminView = false;

    // Dọn sạch dữ liệu cũ khỏi màn hình (tránh lóe lên ở lần đăng nhập sau)
    expenses = [];
    if (chartInstance) {
        chartInstance.destroy();
        chartInstance = null;
    }
    if (statsChartInstance) {
        statsChartInstance.destroy();
        statsChartInstance = null;
    }
    document.getElementById('transactionsList').innerHTML = '';
    document.getElementById('totalIncome').innerText = '0đ';
    document.getElementById('totalExpense').innerText = '0đ';
    document.getElementById('netBalance').innerText = '0đ';
    if (document.getElementById('adminUsersList')) {
        document.getElementById('adminUsersList').innerHTML = '';
    }
    clearChat(); // Dọn sạch tin nhắn

    // Reset về form đăng nhập
    document.getElementById('loginForm').style.display = 'block';
    document.getElementById('registerForm').style.display = 'none';

    // Đưa ứng dụng về màn hình đăng nhập
    authScreen.style.display = 'flex';
    appUI.style.display = 'none';

    // Đưa giao diện về tab Dashboard mặc định
    switchTab('dashboard');
}

function checkLogin() {
    const token = localStorage.getItem('jwt_token');
    if (token) {
        authScreen.style.display = 'none';
        appUI.style.display = 'flex';
        document.getElementById('userNameDisplay').innerText = localStorage.getItem('full_name') || 'User';
        
        if (localStorage.getItem('role') === 'admin') {
            const navAdminBtn = document.getElementById('navAdminBtn');
            if (navAdminBtn) navAdminBtn.style.display = 'block';
        } else {
            const navAdminBtn = document.getElementById('navAdminBtn');
            if (navAdminBtn) navAdminBtn.style.display = 'none';
        }

        fetchData();
    } else {
        authScreen.style.display = 'flex';
        appUI.style.display = 'none';
    }
}

// Tab Switching
function switchTab(tabId, event) {
    // Cập nhật class active cho cả mobile tabs và desktop nav
    document.querySelectorAll('.tab-btn, .nav-btn').forEach(el => el.classList.remove('active'));
    
    // Tìm và active đúng nút theo data-tab
    document.querySelectorAll(`[data-tab="${tabId}"]`).forEach(el => el.classList.add('active'));

    const isDesktop = window.innerWidth > 768;

    if (isDesktop) {
        // Desktop: Chat luôn hiển thị bên phải, chỉ thay đổi panel bên trái
        document.getElementById('dashboardTab').style.display = 'none';
        document.getElementById('statsTab').style.display = 'none';
        document.getElementById('adminTab').style.display = 'none';
        
        if (tabId === 'chat') {
            document.getElementById('dashboardTab').style.display = 'flex'; // Mặc định nếu click chat ở đâu đó
        } else {
            document.getElementById(tabId + 'Tab').style.display = 'flex';
        }
        document.getElementById('chatTab').style.display = 'flex';
    } else {
        // Mobile: Ẩn tất cả và chỉ hiện 1 panel
        document.querySelectorAll('.dashboard-panel, .chat-panel, .admin-panel').forEach(el => {
            el.classList.remove('active-tab');
            el.style.display = 'none';
        });
        const targetTab = document.getElementById(tabId + 'Tab');
        targetTab.classList.add('active-tab');
        targetTab.style.display = 'flex';
    }

    if (tabId === 'admin') fetchAdminData();
    if (tabId === 'stats') {
        initStatsFilters();
        updateStats();
    }
}

let statsChartInstance = null;

function initStatsFilters() {
    const yearSelect = document.getElementById('statsYear');
    const years = new Set();
    if (expenses && expenses.length > 0) {
        expenses.forEach(exp => {
            if (exp.date) years.add(new Date(exp.date).getFullYear());
        });
    }
    
    // Đảm bảo luôn có năm hiện tại
    years.add(new Date().getFullYear());
    
    // Luôn render lại danh sách năm để tránh lưu dữ liệu user cũ
    yearSelect.innerHTML = '';
    Array.from(years).sort((a,b)=>b-a).forEach(y => {
        yearSelect.innerHTML += `<option value="${y}">Năm ${y}</option>`;
    });
    
    // Mặc định trình duyệt sẽ giữ nguyên tháng đã chọn hoặc lấy "all" (Cả năm)
}

function updateStats() {
    const monthVal = document.getElementById('statsMonth').value;
    const yearVal = document.getElementById('statsYear').value;
    
    if (!yearVal) return;

    let filtered = expenses.filter(exp => {
        if (!exp.date) return false;
        const d = new Date(exp.date);
        if (d.getFullYear().toString() !== yearVal) return false;
        if (monthVal !== 'all' && d.getMonth().toString() !== monthVal) return false;
        return true;
    });

    let sIncome = 0;
    let sExpense = 0;
    const sCategory = {};

    filtered.forEach(exp => {
        if (exp.type === 'Thu' || exp.type === 'thu') {
            sIncome += exp.amount;
        } else {
            sExpense += exp.amount;
            sCategory[exp.category] = (sCategory[exp.category] || 0) + exp.amount;
        }
    });

    const formatVND = (num) => new Intl.NumberFormat('vi-VN', { style: 'currency', currency: 'VND' }).format(num);
    document.getElementById('statsIncome').innerText = formatVND(sIncome);
    document.getElementById('statsExpense').innerText = formatVND(sExpense);
    document.getElementById('statsBalance').innerText = formatVND(sIncome - sExpense);

    // Vẽ biểu đồ
    const ctx = document.getElementById('statsChart');
    if (!ctx) return;
    if (statsChartInstance) {
        statsChartInstance.destroy();
    }

    if (Object.keys(sCategory).length === 0) {
        statsChartInstance = new Chart(ctx, {
            type: 'doughnut',
            data: { labels: ['Chưa có dữ liệu'], datasets: [{ data: [1], backgroundColor: ['#ccc'] }] }
        });
        return;
    }

    statsChartInstance = new Chart(ctx, {
        type: 'doughnut',
        data: {
            labels: Object.keys(sCategory),
            datasets: [{
                data: Object.values(sCategory),
                backgroundColor: ['#ef4444', '#f59e0b', '#3b82f6', '#8b5cf6', '#10b981', '#ec4899', '#6366f1'],
                borderWidth: 0
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            plugins: {
                legend: { position: 'right', labels: { color: 'var(--text-primary)' } }
            }
        }
    });
}

// Fetch Data
const fetchData = async () => {
    const token = localStorage.getItem('jwt_token');
    try {
        const response = await fetch('/api/expenses', {
            headers: { 'Authorization': 'Bearer ' + token }
        });
        if (response.status === 401) {
            logout();
            return;
        }
        const data = await response.json();
        expenses = data || [];
        updateDashboard();
    } catch (error) {
        console.error('Lỗi fetch data:', error);
    }
};

// Admin Panel Logic
async function fetchAdminData() {
    const token = localStorage.getItem('jwt_token');
    try {
        const res = await fetch('/api/admin/users', { headers: { 'Authorization': 'Bearer ' + token } });
        const users = await res.json();
        
        const listEl = document.getElementById('adminUserList');
        listEl.innerHTML = '';
        if (!users || users.length === 0) return;

        users.forEach(u => {
            const item = document.createElement('div');
            item.className = 'transaction-item glass-card';
            const roleBadge = u.role === 'admin' ? '<span class="role-badge">Admin</span>' : '';
            item.innerHTML = `
                <div>
                    <strong style="display:block">${u.full_name} ${roleBadge}</strong>
                    <small style="color:var(--text-secondary)">@${u.username} • ${u.tx_count} giao dịch</small>
                </div>
                ${u.role !== 'admin' ? `<button class="del-btn" onclick="deleteUser(${u.id})">Xóa</button>` : ''}
            `;
            listEl.appendChild(item);
        });
    } catch (e) {
        console.error("Lỗi lấy data admin", e);
    }
}

async function deleteUser(id) {
    if (!confirm("Bạn có chắc chắn muốn xóa user này và toàn bộ dữ liệu của họ?")) return;
    const token = localStorage.getItem('jwt_token');
    try {
        const res = await fetch('/api/admin/users', {
            method: 'DELETE',
            headers: { 'Authorization': 'Bearer ' + token, 'Content-Type': 'application/json' },
            body: JSON.stringify({id: id})
        });
        const data = await res.json();
        if (data.success) {
            alert('Đã xóa thành công!');
            fetchAdminData();
        } else {
            alert('Lỗi: ' + data.error);
        }
    } catch (e) {
        alert('Lỗi kết nối máy chủ');
    }
}

// Update Dashboard
const updateDashboard = () => {
    let totalIncome = 0;
    let totalExpense = 0;
    const categoryTotals = {};
    
    const now = new Date();
    const currentMonth = now.getMonth();
    const currentYear = now.getFullYear();
    document.getElementById('currentMonthLabel').innerText = `(Tháng ${currentMonth + 1}/${currentYear})`;

    // Lọc dữ liệu chỉ lấy tháng hiện tại
    const currentMonthExpenses = expenses.filter(exp => {
        if (!exp.date) return false;
        const d = new Date(exp.date);
        return d.getMonth() === currentMonth && d.getFullYear() === currentYear;
    });

    currentMonthExpenses.forEach(exp => {
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
    
    const listEl = document.getElementById('transactionsList');
    listEl.innerHTML = '';
    
    document.getElementById('txCount').innerText = currentMonthExpenses.length;
    
    if (currentMonthExpenses.length === 0) {
        listEl.innerHTML = '<div class="loading">Tháng này chưa có giao dịch nào.</div>';
    }

    const recent = [...currentMonthExpenses].reverse();
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
            datasets: [{ data: data, backgroundColor: bgColors, borderWidth: 0 }]
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
    
    appendMessage("⏳ AI đang ghi sổ...", false);
    const typingMsg = chatHistory.lastElementChild;

    try {
        const token = localStorage.getItem('jwt_token');
        const res = await fetch('/api/chat', {
            method: 'POST',
            headers: { 'Authorization': 'Bearer ' + token },
            body: formData
        });
        
        if (res.status === 401) {
            typingMsg.remove();
            appendMessage("❌ Phiên đăng nhập hết hạn, vui lòng đăng xuất và đăng nhập lại.", false);
            return;
        }

        const data = await res.json();
        typingMsg.remove();
        
        if (data.success) {
            appendMessage(data.reply, false);
            if (data.expenses && data.expenses.length > 0) {
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

// Khởi tạo ban đầu
checkLogin();

// Hàm dọn dẹp tin nhắn
function clearChat() {
    const chatHistory = document.getElementById('chatHistory');
    chatHistory.innerHTML = `
        <div class="message ai-message">
            <p>Xin chào! Tôi là trợ lý tài chính AI. Lịch sử trò chuyện đã được dọn dẹp sạch sẽ 🧹. Bạn có khoản thu chi nào mới không?</p>
        </div>
    `;
    // Thêm hiệu ứng nháy nhẹ để người dùng biết đã dọn
    chatHistory.style.opacity = '0.5';
    setTimeout(() => { chatHistory.style.opacity = '1'; }, 200);
}
