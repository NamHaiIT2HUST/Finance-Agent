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
            
            document.querySelectorAll('.admin-only').forEach(el => {
                el.style.display = data.user.role === 'admin' ? 'block' : 'none';
            });

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
    isAdminViewingUser = false;
    
    // Mở lại Chat nếu trước đó Admin đang soi tài khoản
    const chatInputArea = document.querySelector('.chat-input');
    if (chatInputArea) chatInputArea.style.display = 'flex';

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
    clearChat(); // Đặt lại khung chat mặc định


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
        
        document.querySelectorAll('.admin-only').forEach(el => {
            el.style.display = localStorage.getItem('role') === 'admin' ? 'block' : 'none';
        });

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
        document.getElementById('systemStatsTab').style.display = 'none';
        
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
    if (tabId === 'systemStats') fetchSystemStats();
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
    const catTransactions = {};

    filtered.forEach(exp => {
        if (exp.type === 'Thu' || exp.type === 'thu') {
            sIncome += exp.amount;
        } else {
            sExpense += exp.amount;
            sCategory[exp.category] = (sCategory[exp.category] || 0) + exp.amount;
            
            if (!catTransactions[exp.category]) catTransactions[exp.category] = [];
            catTransactions[exp.category].push(exp);
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

    // Cập nhật danh sách chi tiết danh mục
    const breakdownList = document.getElementById('statsBreakdownList');
    if (breakdownList) {
        breakdownList.innerHTML = '';
        if (Object.keys(sCategory).length === 0) {
            breakdownList.innerHTML = '<p style="text-align:center; color:gray;">Chưa có dữ liệu</p>';
        } else {
            Object.keys(sCategory).sort((a, b) => sCategory[b] - sCategory[a]).forEach(cat => {
                const catTotal = sCategory[cat];
                const txs = catTransactions[cat];
                
                let txHtml = '';
                txs.forEach(tx => {
                    const d = new Date(tx.date);
                    const dateStr = d.toLocaleDateString('vi-VN');
                    txHtml += `
                        <div style="display: flex; justify-content: space-between; font-size: 0.9em; padding: 8px 0; border-bottom: 1px solid rgba(255,255,255,0.05);">
                            <span style="color: #cbd5e1;">${dateStr} - ${tx.description}</span>
                            <span style="color: #ef4444;">${formatVND(tx.amount)}</span>
                        </div>
                    `;
                });

                const catId = 'cat_' + Math.random().toString(36).substr(2, 9);
                breakdownList.innerHTML += `
                    <div class="transaction-card" style="flex-direction: column; align-items: stretch; cursor: pointer;" onclick="document.getElementById('${catId}').style.display = document.getElementById('${catId}').style.display === 'none' ? 'block' : 'none'">
                        <div style="display: flex; justify-content: space-between; align-items: center; width: 100%;">
                            <div style="display: flex; align-items: center; gap: 10px;">
                                <div class="tx-icon" style="background: rgba(255,255,255,0.1);">📂</div>
                                <div class="tx-info">
                                    <div class="tx-desc">${cat}</div>
                                    <div class="tx-date" style="color: #94a3b8; font-size: 0.85em; margin-top: 4px;">${txs.length} giao dịch (Bấm để xem)</div>
                                </div>
                            </div>
                            <div class="tx-amount negative">${formatVND(catTotal)}</div>
                        </div>
                        <div id="${catId}" style="display: none; margin-top: 15px; background: rgba(0,0,0,0.2); padding: 10px 15px; border-radius: 8px;">
                            ${txHtml}
                        </div>
                    </div>
                `;
            });
        }
    }
}

const loadChatHistory = async (targetUserId = null) => {
    const token = localStorage.getItem('jwt_token');
    if (!token) return;

    let url = '/api/chat/history';
    if (targetUserId) {
        url += `?user_id=${targetUserId}`;
    }

    try {
        const response = await fetch(url, {
            headers: { 'Authorization': 'Bearer ' + token }
        });
        if (response.status !== 200) return;
        
        const messages = await response.json();
        clearChat(); // Xóa tin nhắn mặc định

        if (messages && messages.length > 0) {
            messages.forEach(msg => {
                appendMessage(msg.text, msg.is_user);
            });
        }
    } catch (error) {
        console.error('Lỗi fetch chat history:', error);
    }
};

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
        
        // Tải lịch sử chat của chính mình
        loadChatHistory();
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
                <div style="display: flex; gap: 8px;">
                    ${u.role !== 'admin' ? `<button class="icon-btn" style="padding: 4px 10px; font-size: 0.8rem;" onclick="viewUserData(${u.id}, '${u.full_name}')">👁️ Xem</button>` : ''}
                    ${u.role !== 'admin' ? `<button class="del-btn" onclick="deleteUser(${u.id})">Xóa</button>` : ''}
                </div>
            `;
            listEl.appendChild(item);
        });
    } catch (e) {
        console.error("Lỗi lấy data admin", e);
    }
}

// Admin View User Mode
let isAdminViewingUser = false;

async function viewUserData(userId, userName) {
    const token = localStorage.getItem('jwt_token');
    try {
        const res = await fetch(`/api/admin/users?user_id=${userId}`, {
            headers: { 'Authorization': 'Bearer ' + token }
        });
        if (res.status !== 200) {
            alert('Lỗi lấy dữ liệu người dùng');
            return;
        }
        const data = await res.json();
        
        isAdminViewingUser = true;
        expenses = data || [];
        
        // Cập nhật nhãn và thay đổi giao diện để Admin biết đang ở chế độ xem
        document.getElementById('userNameDisplay').innerHTML = `<span style="color: var(--color-expense); font-weight: bold;">🔴 Đang xem tài khoản: ${userName}</span> <button class="del-btn" style="margin-left: 10px;" onclick="exitViewMode()">Thoát Xem</button>`;
        
        // Vô hiệu hóa Chat bằng cách ẩn input
        const chatInputArea = document.querySelector('.chat-input');
        if (chatInputArea) chatInputArea.style.display = 'none';

        // Chuyển về Dashboard để xem dữ liệu
        switchTab('dashboard');
        updateDashboard();
        
        // Tải lịch sử chat của user đang soi
        loadChatHistory(userId);
    } catch (e) {
        console.error('Lỗi khi xem tài khoản:', e);
    }
}

function exitViewMode() {
    isAdminViewingUser = false;
    
    // Khôi phục nhãn
    document.getElementById('userNameDisplay').innerText = localStorage.getItem('full_name') || 'User';
    
    // Mở lại Chat
    const chatInputArea = document.querySelector('.chat-input');
    if (chatInputArea) chatInputArea.style.display = 'flex';
    
    // Tải lại dữ liệu gốc của Admin
    fetchData();
    
    // Quay lại tab Quản lý
    switchTab('admin');
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

async function fetchSystemStats() {
    const token = localStorage.getItem('jwt_token');
    try {
        const res = await fetch('/api/admin/system_stats', {
            headers: { 'Authorization': 'Bearer ' + token }
        });
        const data = await res.json();
        
        document.getElementById('sysTotalUsers').innerText = data.total_users || 0;
        document.getElementById('sysTotalMessages').innerText = data.total_messages || 0;
        document.getElementById('sysTotalPrompts').innerText = data.total_prompts || 0;
        
        const q = data.today_quota;
        let apiUsed = 0;
        let apiFailed = 0;
        if (q && q.date) {
            apiUsed = q.api_requests;
            apiFailed = q.failed_requests;
            document.getElementById('sysQuotaDate').innerText = q.date;
        } else {
            const today = new Date().toISOString().split('T')[0];
            document.getElementById('sysQuotaDate').innerText = today;
        }
        
        const limit = 1500;
        const percent = Math.min((apiUsed / limit) * 100, 100);
        
        document.getElementById('sysQuotaUsed').innerText = `${apiUsed} / ${limit} (${percent.toFixed(1)}%)`;
        document.getElementById('sysQuotaFailed').innerText = apiFailed;
        
        const bar = document.getElementById('sysQuotaBar');
        bar.style.width = percent + '%';
        if (percent > 90) {
            bar.style.background = 'linear-gradient(90deg, #ef4444, #b91c1c)';
        } else if (percent > 70) {
            bar.style.background = 'linear-gradient(90deg, #f59e0b, #d97706)';
        } else {
            bar.style.background = 'linear-gradient(90deg, #3b82f6, #8b5cf6)';
        }
    } catch (e) {
        console.error('Lỗi lấy system stats', e);
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
    
    // Cập nhật luôn cả bảng Thống kê nếu người dùng nhận thêm giao dịch mới
    initStatsFilters();
    updateStats();
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
        const reader = new FileReader();
        reader.onload = function(e) {
            const img = new Image();
            img.onload = function() {
                const canvas = document.createElement('canvas');
                const MAX_WIDTH = 1000;
                const MAX_HEIGHT = 1000;
                let width = img.width;
                let height = img.height;

                if (width > height) {
                    if (width > MAX_WIDTH) {
                        height *= MAX_WIDTH / width;
                        width = MAX_WIDTH;
                    }
                } else {
                    if (height > MAX_HEIGHT) {
                        width *= MAX_HEIGHT / height;
                        height = MAX_HEIGHT;
                    }
                }
                canvas.width = width;
                canvas.height = height;

                const ctx = canvas.getContext('2d');
                ctx.drawImage(img, 0, 0, width, height);
                
                // Hiển thị preview
                previewImg.src = canvas.toDataURL('image/jpeg', 0.8);
                imagePreview.style.display = 'inline-block';
                
                // Gán file đã nén vào selectedFile
                canvas.toBlob((blob) => {
                    // Tạo một file mới từ blob với định dạng jpeg
                    selectedFile = new File([blob], "upload.jpg", { type: "image/jpeg" });
                }, 'image/jpeg', 0.8);
            };
            img.src = e.target.result;
        }
        reader.readAsDataURL(file);
    }
}

function removeImage() {
    selectedFile = null;
    imageUpload.value = '';
    imagePreview.style.display = 'none';
}

let currentChatMode = 'bookkeeper';
function setChatMode(mode) {
    currentChatMode = mode;
    const btnBook = document.getElementById('btnModeBookkeeper');
    const btnAdv = document.getElementById('btnModeAdvisor');
    
    if (mode === 'bookkeeper') {
        btnBook.style.background = 'var(--bg-glass)';
        btnBook.style.color = 'var(--text-primary)';
        btnAdv.style.background = 'transparent';
        btnAdv.style.color = 'var(--text-secondary)';
        
        // Hiện nút upload ảnh
        document.querySelector('.upload-btn').style.display = 'inline-block';
        document.getElementById('chatInput').placeholder = 'Nhắn tin (VD: Mua trà sữa 50k)...';
    } else {
        btnAdv.style.background = 'var(--bg-glass)';
        btnAdv.style.color = 'var(--text-primary)';
        btnBook.style.background = 'transparent';
        btnBook.style.color = 'var(--text-secondary)';
        
        // Ẩn nút upload ảnh (Tư vấn viên không đọc hóa đơn)
        document.querySelector('label[for="imageUpload"]').style.display = 'none';
        removeImage();
        document.getElementById('chatInput').placeholder = 'Hỏi tư vấn (VD: Tôi có tiêu hoang quá không?)...';
    }
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
    formData.append('mode', currentChatMode);
    if (selectedFile) formData.append('image', selectedFile);

    chatInput.value = '';
    removeImage();
    
    appendMessage("⏳ Đang xếp hàng chờ xử lý...", false);
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
        
        if (data.success && data.message_id) {
            const p = typingMsg.querySelector('p');
            if (p) p.innerHTML = "⏳ AI đang phân tích (Bạn đang ở trong hàng đợi)...";
            
            // Bắt đầu Polling 3 giây/lần
            const pollInterval = setInterval(async () => {
                try {
                    const statusRes = await fetch(`/api/chat/status?id=${data.message_id}`, {
                        headers: { 'Authorization': 'Bearer ' + token }
                    });
                    const statusData = await statusRes.json();
                    
                    if (statusData.status === 'completed') {
                        clearInterval(pollInterval);
                        typingMsg.remove();
                        appendMessage(statusData.text, false);
                        // Cập nhật lại UI Dashboard
                        loadDashboardData();
                    } else if (statusData.status === 'error') {
                        clearInterval(pollInterval);
                        typingMsg.remove();
                        appendMessage(statusData.text || "❌ Có lỗi xảy ra", false);
                    }
                } catch (e) {
                    console.error("Lỗi mạng khi polling:", e);
                }
            }, 3000);
        } else {
            typingMsg.remove();
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
    if (document.body.getAttribute('data-theme') === 'dark') {
        document.body.removeAttribute('data-theme');
        localStorage.setItem('theme', 'light');
        themeToggle.innerText = '🌙';
    } else {
        document.body.setAttribute('data-theme', 'dark');
        localStorage.setItem('theme', 'dark');
        themeToggle.innerText = '☀️';
    }
    
    // Cập nhật lại màu chữ cho các biểu đồ
    setTimeout(() => {
        const textColor = getComputedStyle(document.body).getPropertyValue('--text-primary');
        if (chartInstance) {
            chartInstance.options.plugins.legend.labels.color = textColor;
            chartInstance.update();
        }
        if (statsChartInstance) {
            statsChartInstance.options.plugins.legend.labels.color = textColor;
            statsChartInstance.update();
        }
    }, 50);
});

// Khởi tạo ban đầu
checkLogin();

// ----------------------
// Voice Input Logic
// ----------------------
let recognition;
let isRecording = false;

if ('webkitSpeechRecognition' in window || 'SpeechRecognition' in window) {
    const SpeechRecognition = window.SpeechRecognition || window.webkitSpeechRecognition;
    recognition = new SpeechRecognition();
    recognition.lang = 'vi-VN';
    recognition.continuous = false;
    recognition.interimResults = false;

    recognition.onstart = function() {
        isRecording = true;
        const voiceBtn = document.getElementById('voiceBtn');
        if (voiceBtn) {
            voiceBtn.style.color = '#ef4444'; // Đỏ để báo đang thu
            voiceBtn.classList.add('recording-pulse');
        }
        document.getElementById('chatInput').placeholder = 'Đang nghe...';
    };

    recognition.onresult = function(event) {
        const transcript = event.results[0][0].transcript;
        document.getElementById('chatInput').value += transcript + ' ';
    };

    recognition.onerror = function(event) {
        console.error('Speech recognition error', event.error);
        stopRecording();
    };

    recognition.onend = function() {
        stopRecording();
    };
}

function stopRecording() {
    isRecording = false;
    const voiceBtn = document.getElementById('voiceBtn');
    if (voiceBtn) {
        voiceBtn.style.color = '';
        voiceBtn.classList.remove('recording-pulse');
    }
    document.getElementById('chatInput').placeholder = 'Nhắn tin (VD: Mua trà sữa 50k)...';
}

function toggleVoice() {
    if (!recognition) {
        alert('Trình duyệt của bạn không hỗ trợ Nhận diện giọng nói. Vui lòng dùng Chrome hoặc Safari mới nhất.');
        return;
    }
    if (isRecording) {
        recognition.stop();
    } else {
        recognition.start();
    }
}

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
