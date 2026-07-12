document.addEventListener('DOMContentLoaded', () => {
    // Khởi tạo Telegram Web App
    const tg = window.Telegram.WebApp;
    tg.expand();

    // Theme logic
    const themeBtn = document.getElementById('themeToggle');
    const root = document.documentElement;
    const body = document.body;

    // Lấy theme mặc định từ Telegram hoặc localStorage
    let currentTheme = localStorage.getItem('theme') || (tg.colorScheme === 'dark' ? 'dark' : 'light');
    if (currentTheme === 'dark') {
        root.setAttribute('data-theme', 'dark');
        themeBtn.textContent = '☀️';
    }

    themeBtn.addEventListener('click', () => {
        if (root.getAttribute('data-theme') === 'dark') {
            root.removeAttribute('data-theme');
            localStorage.setItem('theme', 'light');
            themeBtn.textContent = '🌙';
        } else {
            root.setAttribute('data-theme', 'dark');
            localStorage.setItem('theme', 'dark');
            themeBtn.textContent = '☀️';
        }
        if (expenseChart) updateChartTheme();
    });

    // Cập nhật tên user từ Telegram
    const userNameEl = document.getElementById('userName');
    if (tg.initDataUnsafe && tg.initDataUnsafe.user) {
        userNameEl.textContent = `Xin chào, ${tg.initDataUnsafe.user.first_name}!`;
    }

    // Biến lưu trữ
    let expensesData = [];
    let expenseChart = null;

    // Format tiền tệ VNĐ
    const formatCurrency = (amount) => {
        return new Intl.NumberFormat('vi-VN', { style: 'currency', currency: 'VND' }).format(amount);
    };

    // Hàm gọi API lấy dữ liệu
    const fetchData = async () => {
        try {
            // Trong môi trường production, API sẽ cùng host với Web App
            const response = await fetch('/api/expenses');
            if (!response.ok) throw new Error('Network response was not ok');
            
            expensesData = await response.json();
            
            if (!expensesData) expensesData = [];
            
            // Xử lý dữ liệu fallback "Chi"
            expensesData = expensesData.map(e => ({
                ...e,
                type: (e.type && e.type !== "") ? e.type : "Chi"
            }));

            updateDashboard();
            renderTransactions();
            renderChart();
        } catch (error) {
            console.error('Error fetching expenses:', error);
            document.getElementById('transactionsList').innerHTML = `<div class="loading">❌ Lỗi tải dữ liệu. Vui lòng thử lại sau.</div>`;
        }
    };

    const updateDashboard = () => {
        let totalIncome = 0;
        let totalExpense = 0;

        expensesData.forEach(exp => {
            const isThu = exp.type.toLowerCase() === 'thu';
            if (isThu) {
                totalIncome += exp.amount;
            } else {
                totalExpense += exp.amount;
            }
        });

        const balance = totalIncome - totalExpense;

        document.getElementById('totalIncome').textContent = formatCurrency(totalIncome);
        document.getElementById('totalExpense').textContent = formatCurrency(totalExpense);
        document.getElementById('netBalance').textContent = formatCurrency(balance);
        document.getElementById('txCount').textContent = expensesData.length;
    };

    const renderTransactions = () => {
        const listEl = document.getElementById('transactionsList');
        listEl.innerHTML = '';

        if (expensesData.length === 0) {
            listEl.innerHTML = '<div class="loading">Chưa có giao dịch nào.</div>';
            return;
        }

        // Đảo ngược mảng để giao dịch mới nhất lên đầu (hoặc lấy 10 cái mới nhất)
        const recentTx = [...expensesData].reverse().slice(0, 10);

        recentTx.forEach(exp => {
            const isThu = exp.type.toLowerCase() === 'thu';
            
            const item = document.createElement('div');
            item.className = 'tx-item';
            
            item.innerHTML = `
                <div class="tx-left">
                    <div class="tx-icon ${isThu ? 'thu' : 'chi'}">
                        ${isThu ? '↓' : '↑'}
                    </div>
                    <div class="tx-details">
                        <h4>${exp.description}</h4>
                        <p>${exp.category} • ${exp.date}</p>
                    </div>
                </div>
                <div class="tx-right">
                    <span class="tx-amount ${isThu ? 'positive' : 'negative'}">
                        ${isThu ? '+' : '-'}${formatCurrency(exp.amount)}
                    </span>
                </div>
            `;
            listEl.appendChild(item);
        });
    };

    const renderChart = () => {
        const ctx = document.getElementById('expenseChart').getContext('2d');
        
        // Nhóm dữ liệu Chi
        const categories = {};
        expensesData.forEach(exp => {
            if (exp.type.toLowerCase() !== 'thu') {
                const cat = exp.category || 'Khác';
                categories[cat] = (categories[cat] || 0) + exp.amount;
            }
        });

        const labels = Object.keys(categories);
        const data = Object.values(categories);

        // Màu sắc hiện đại
        const backgroundColors = [
            '#3b82f6', '#ef4444', '#10b981', '#f59e0b', '#8b5cf6', '#ec4899', '#14b8a6'
        ];

        const isDark = root.getAttribute('data-theme') === 'dark';
        const textColor = isDark ? '#f8fafc' : '#111827';

        if (expenseChart) expenseChart.destroy();

        expenseChart = new Chart(ctx, {
            type: 'doughnut',
            data: {
                labels: labels,
                datasets: [{
                    data: data,
                    backgroundColor: backgroundColors,
                    borderWidth: 0,
                    hoverOffset: 10
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                cutout: '70%',
                plugins: {
                    legend: {
                        position: 'right',
                        labels: {
                            color: textColor,
                            font: { family: 'Inter', size: 12 }
                        }
                    }
                }
            }
        });
    };

    const updateChartTheme = () => {
        if (!expenseChart) return;
        const isDark = root.getAttribute('data-theme') === 'dark';
        expenseChart.options.plugins.legend.labels.color = isDark ? '#f8fafc' : '#111827';
        expenseChart.update();
    };

    // Khởi chạy
    fetchData();
});
