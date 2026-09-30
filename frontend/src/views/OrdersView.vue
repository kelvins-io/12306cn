<template>
  <div class="page">
    <el-card shadow="never">
      <template #header>
        <div class="hdr">
          <span>我的订单</span>
          <el-button @click="load" :loading="loading">刷新</el-button>
        </div>
      </template>
      <el-table :data="list" v-loading="loading">
        <el-table-column prop="order_no" label="订单号" min-width="140" />
        <el-table-column label="行程" min-width="200">
          <template #default="{ row }">
            <div>{{ row.train_no }}　{{ row.travel_date }}</div>
            <div>{{ row.from_station }} → {{ row.to_station }} · {{ row.seat_type }}</div>
            <div v-if="row.transfer_group_id" class="muted">中转第 {{ row.transfer_leg }} 程（组 {{ row.transfer_group_id.slice(0, 8) }}）</div>
            <div v-if="row.parent_order_id" class="muted">改签自订单 #{{ row.parent_order_id }}（第 {{ row.reschedule_count || 1 }} 次）</div>
            <div v-if="row.price_diff" class="muted">
              改签差价：{{ row.price_diff > 0 ? `应退 ¥${(row.price_diff / 100).toFixed(2)}` : `需补 ¥${(-row.price_diff / 100).toFixed(2)}` }}
            </div>
          </template>
        </el-table-column>
        <el-table-column label="乘客/座位" min-width="220">
          <template #default="{ row }">
            <div v-for="t in row.tickets || []" :key="t.id">
              {{ t.passenger_name }}　{{ t.carriage_no }}车{{ t.seat_no }}
              <span v-if="t.verify_code" class="muted"> 取票码 {{ t.verify_code }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="金额" width="100">
          <template #default="{ row }">
            <span class="money">¥{{ (row.total_amount / 100).toFixed(2) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="110">
          <template #default="{ row }">
            <el-tag :type="statusType(row.status)">{{ statusText(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="240" fixed="right">
          <template #default="{ row }">
            <el-button v-if="row.status === 'pending_pay'" type="primary" size="small" @click="pay(row)">支付</el-button>
            <el-button v-if="row.status === 'pending_pay'" size="small" @click="cancel(row)">取消</el-button>
            <el-button v-if="row.status === 'paid'" type="warning" size="small" @click="openReschedule(row)">改签</el-button>
            <el-button v-if="row.status === 'paid'" type="danger" size="small" @click="refund(row)">退票</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="rsVisible" title="改签" width="560px" @open="onRsOpen">
      <el-form label-width="80px">
        <el-form-item label="出发">
          <el-input v-model="rsForm.from" />
        </el-form-item>
        <el-form-item label="到达">
          <el-input v-model="rsForm.to" />
        </el-form-item>
        <el-form-item label="日期">
          <el-date-picker v-model="rsForm.date" type="date" value-format="YYYY-MM-DD" style="width:100%" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="rsSearching" @click="searchRs">查询车次</el-button>
        </el-form-item>
      </el-form>
      <el-table :data="rsList" height="260" @row-click="(row) => (rsPick = row)" highlight-current-row>
        <el-table-column prop="train_no" label="车次" width="90" />
        <el-table-column label="时刻" min-width="140">
          <template #default="{ row }">{{ row.depart_time }} → {{ row.arrive_time }}</template>
        </el-table-column>
        <el-table-column label="席别" min-width="180">
          <template #default="{ row }">
            <el-radio-group v-model="rsSeatType" size="small">
              <el-radio
                v-for="(n, t) in row.seat_remaining"
                :key="t"
                :value="t"
                :disabled="n <= 0"
                @click.stop
              >{{ t }}({{ n }})</el-radio>
            </el-radio-group>
          </template>
        </el-table-column>
      </el-table>
      <template #footer>
        <el-button @click="rsVisible = false">取消</el-button>
        <el-button type="primary" :loading="rsSubmitting" @click="submitRs">确认改签</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import * as api from '../api'

const list = ref([])
const loading = ref(false)
const rsVisible = ref(false)
const rsSearching = ref(false)
const rsSubmitting = ref(false)
const rsCurrent = ref(null)
const rsList = ref([])
const rsPick = ref(null)
const rsSeatType = ref('')
const rsForm = reactive({ from: '', to: '', date: '' })

const statusMap = {
  pending_pay: '待支付',
  paid: '已出票',
  cancelled: '已取消',
  refunded: '已退票',
  expired: '已过期',
  rescheduled: '已改签',
}
function statusText(s) {
  return statusMap[s] || s
}
function statusType(s) {
  return {
    pending_pay: 'warning',
    paid: 'success',
    cancelled: 'info',
    refunded: 'danger',
    expired: 'info',
    rescheduled: 'info',
  }[s] || ''
}

async function load() {
  loading.value = true
  try {
    list.value = await api.listOrders()
  } finally {
    loading.value = false
  }
}

async function pay(row) {
  await api.payOrder(row.id)
  ElMessage.success('支付成功')
  load()
}

async function cancel(row) {
  await ElMessageBox.confirm('确认取消该订单？', '提示')
  await api.cancelOrder(row.id)
  ElMessage.success('已取消')
  load()
}

async function refund(row) {
  await ElMessageBox.confirm('确认退票？座位将释放并可触发候补兑现。', '退票')
  await api.refundOrder(row.id)
  ElMessage.success('退票成功')
  load()
}

function openReschedule(row) {
  rsCurrent.value = row
  rsForm.from = row.from_station
  rsForm.to = row.to_station
  rsForm.date = row.travel_date
  rsSeatType.value = row.seat_type
  rsList.value = []
  rsPick.value = null
  rsVisible.value = true
}

function onRsOpen() {
  searchRs()
}

async function searchRs() {
  rsSearching.value = true
  try {
    rsList.value = await api.queryTickets({ from: rsForm.from, to: rsForm.to, date: rsForm.date })
    if (rsList.value.length) {
      rsPick.value = rsList.value[0]
      const rem = rsPick.value.seat_remaining || {}
      if (!rem[rsSeatType.value] || rem[rsSeatType.value] <= 0) {
        rsSeatType.value = Object.keys(rem).find((k) => rem[k] > 0) || rsSeatType.value
      }
    }
  } finally {
    rsSearching.value = false
  }
}

async function submitRs() {
  if (!rsPick.value || !rsSeatType.value) {
    ElMessage.warning('请选择车次和席别')
    return
  }
  await ElMessageBox.confirm(
    '改签规则：最多 2 次；发车前 2 小时截止；低改高需补差价支付，高改低直接出票并记录应退差价。确认继续？',
    '改签确认'
  )
  rsSubmitting.value = true
  try {
    const order = await api.rescheduleOrder(rsCurrent.value.id, {
      train_id: rsPick.value.train_id,
      travel_date: rsForm.date,
      from_station: rsPick.value.from_station,
      to_station: rsPick.value.to_station,
      seat_type: rsSeatType.value,
    })
    ElMessage.success(`改签成功，新订单 ${order.order_no}`)
    rsVisible.value = false
    load()
  } finally {
    rsSubmitting.value = false
  }
}

load()
</script>

<style scoped>
.hdr {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.muted {
  color: #999;
  font-size: 12px;
}
</style>
