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
                document.getElementById('adminTabBtn').style.display = 'block';
                document.getElementById('adminDesktopBtn').style.display = 'block';
            } else {
                document.getElementById('adminTabBtn').style.display = 'none';
                document.getElementById('adminDesktopBtn').style.display = 'none';
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
    document.getElementById('dashboardTab').style.display = '';
    document.getElementById('chatTab').style.display = '';
    document.getElementById('adminTab').style.display = 'none';
    document.getElementById('adminTab').style.flex = '';

    authScreen.style.display = 'flex';
    appUI.style.display = 'none';
}

function checkLogin() {
    const token = localStorage.getItem('jwt_token');
    if (token) {
        authScreen.style.display = 'none';
        appUI.style.display = 'flex';
        document.getElementById('userNameDisplay').innerText = localStorage.getItem('full_name') || 'User';
        
        if (localStorage.getItem('role') === 'admin') {
            document.getElementById('adminTabBtn').style.display = 'block';
            document.getElementById('adminDesktopBtn').style.display = 'block';
        } else {
            document.getElementById('adminTabBtn').style.display = 'none';
            document.getElementById('adminDesktopBtn').style.display = 'none';
        }

        fetchData();
    } else {
        authScreen.style.display = 'flex';
        appUI.style.display = 'none';
    }
}

// Tab Switching
function switchTab(tabId) {
    document.querySelectorAll('.dashboard-panel, .chat-panel, .admin-panel').forEach(el => el.classList.remove('active-tab'));
    document.querySelectorAll('.tab-btn').forEach(el => el.classList.remove('active'));
    
    document.getElementById(tabId + 'Tab').classList.add('active-tab');
    event.target.classList.add('active');

    if (tabId === 'admin') fetchAdminData();
}

let isAdminView = false;
function toggleAdminView() {
    isAdminView = !isAdminView;
    if (isAdminView) {
        document.getElementById('dashboardTab').style.display = 'none';
        document.getElementById('chatTab').style.display = 'none';
        document.getElementById('adminTab').style.display = 'flex';
        document.getElementById('adminTab').style.flex = '1';
        fetchAdminData();
    } else {
        // Đảm bảo dashboard và chat hiện lại (bằng cách xóa inline style)
        document.getElementById('dashboardTab').style.display = '';
        document.getElementById('chatTab').style.display = '';
        // ĐẢM BẢO admin tab bị ẩn đi, không được phép xóa inline style của nó
        document.getElementById('adminTab').style.display = 'none';
        document.getElementById('adminTab').style.flex = '';
    }
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

    const listEl = document.getElementById('transactionsList');
    listEl.innerHTML = '';
    const recent = [...expenses].reverse().slice(0, 15);
    
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
