<template>
  <div class="page">
    <el-card shadow="never" class="search-card">
      <el-form :inline="true" :model="query" @submit.prevent="search">
        <el-form-item label="出发">
          <el-select v-model="query.from" filterable allow-create default-first-option placeholder="车站" style="width:140px">
            <el-option v-for="s in stations" :key="s.id" :label="s.name" :value="s.name" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button circle @click="swap">
            <el-icon><Sort /></el-icon>
          </el-button>
        </el-form-item>
        <el-form-item label="到达">
          <el-select v-model="query.to" filterable allow-create default-first-option placeholder="车站" style="width:140px">
            <el-option v-for="s in stations" :key="s.id" :label="s.name" :value="s.name" />
          </el-select>
        </el-form-item>
        <el-form-item label="日期">
          <el-date-picker v-model="query.date" type="date" value-format="YYYY-MM-DD" :disabled-date="disablePast" />
        </el-form-item>
        <el-form-item label="方式">
          <el-radio-group v-model="query.mode">
            <el-radio-button value="direct">直达</el-radio-button>
            <el-radio-button value="transfer">中转</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="loading" native-type="submit">查询</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <el-card v-if="query.mode === 'direct'" shadow="never" style="margin-top:16px">
      <el-table :data="list" v-loading="loading" empty-text="请查询车票">
        <el-table-column prop="train_no" label="车次" width="100">
          <template #default="{ row }">
            <strong>{{ row.train_no }}</strong>
            <div class="muted">{{ row.train_type }}</div>
          </template>
        </el-table-column>
        <el-table-column label="站点/时刻" min-width="180">
          <template #default="{ row }">
            <div>{{ row.from_station }} {{ row.depart_time }}</div>
            <div>{{ row.to_station }} {{ row.arrive_time }}</div>
            <div class="muted">约 {{ row.duration_min }} 分钟</div>
          </template>
        </el-table-column>
        <el-table-column label="余票" min-width="280">
          <template #default="{ row }">
            <el-tag v-if="row.sale_open === false" type="warning" effect="plain">{{ row.sale_message || '未开售' }}</el-tag>
            <template v-else>
              <el-tag
                v-for="(n, t) in row.seat_remaining"
                :key="t"
                class="seat-tag"
                :type="n > 0 ? 'success' : 'info'"
                effect="plain"
                @click="n > 0 && openBook(row, t)"
              >
                {{ t }} {{ n > 0 ? n : '无' }}
                <span v-if="row.seat_price?.[t]" class="money"> ¥{{ (row.seat_price[t] / 100).toFixed(0) }}</span>
                <span class="muted"> 成人价</span>
              </el-tag>
            </template>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="160" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" size="small" :disabled="row.sale_open === false" @click="openBook(row)">预订</el-button>
            <el-button size="small" :disabled="row.sale_open === false" @click="openWait(row)">候补</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-card v-else shadow="never" style="margin-top:16px">
      <el-table :data="transfers" v-loading="loading" empty-text="暂无同日中转方案（试北京→深圳）">
        <el-table-column label="方案" min-width="280">
          <template #default="{ row }">
            <div>
              <strong>{{ row.leg1.train_no }}</strong> {{ row.leg1.from_station }} {{ row.leg1.depart_time }}
              → {{ row.leg1.to_station }} {{ row.leg1.arrive_time }}
            </div>
            <div class="muted">换乘 {{ row.hub_station }}，候车 {{ row.wait_minutes }} 分钟</div>
            <div>
              <strong>{{ row.leg2.train_no }}</strong> {{ row.leg2.from_station }} {{ row.leg2.depart_time }}
              → {{ row.leg2.to_station }} {{ row.leg2.arrive_time }}
            </div>
            <div class="muted">总历时约 {{ row.total_duration_min }} 分钟</div>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="120" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" size="small" @click="openTransfer(row)">预订中转</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="bookVisible" title="确认订单" width="520px">
      <div v-if="current">
        <p>{{ current.train_no }}　{{ query.date }}　{{ current.from_station }} → {{ current.to_station }}</p>
        <el-form label-width="80px">
          <el-form-item label="席别">
            <el-select v-model="bookForm.seatType" style="width:100%">
              <el-option
                v-for="(n, t) in current.seat_remaining"
                :key="t"
                :label="`${t}（余 ${n}）`"
                :value="t"
                :disabled="n <= 0"
              />
            </el-select>
          </el-form-item>
          <el-form-item label="选座">
            <el-select v-model="bookForm.position" clearable placeholder="不限" style="width:48%;margin-right:4%">
              <el-option label="靠窗" value="靠窗" />
              <el-option label="过道" value="过道" />
              <el-option label="中间" value="中间" />
            </el-select>
            <el-input v-model="bookForm.carriage" placeholder="车厢号如 01" style="width:48%" clearable />
          </el-form-item>
          <el-form-item label="乘车人">
            <el-checkbox-group v-model="bookForm.passengerIds">
              <el-checkbox v-for="p in passengers" :key="p.id" :value="p.id">
                {{ p.name }}（{{ p.passenger_type || '成人' }}）
              </el-checkbox>
            </el-checkbox-group>
            <div class="muted">儿童5折 / 学生7.5折 / 军人5折（按票种计价）</div>
            <div v-if="!passengers.length" class="muted">
              暂无乘车人，请先
              <router-link to="/passengers">添加</router-link>
            </div>
          </el-form-item>
          <el-form-item label="验证码">
            <div class="captcha-row">
              <el-input v-model="bookForm.captcha_code" placeholder="计算结果" style="flex:1" />
              <el-button @click="loadBookCaptcha">{{ bookCaptcha.question || '获取' }}</el-button>
            </div>
          </el-form-item>
        </el-form>
      </div>
      <template #footer>
        <el-button @click="bookVisible = false">取消</el-button>
        <el-button type="primary" :loading="booking" @click="submitBook">提交订单</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="waitVisible" title="候补购票" width="480px">
      <el-form label-width="80px" v-if="current">
        <el-form-item label="席别">
          <el-select v-model="waitForm.seatType" style="width:100%">
            <el-option v-for="(_, t) in current.seat_remaining" :key="t" :label="t" :value="t" />
          </el-select>
        </el-form-item>
        <el-form-item label="乘车人">
          <el-select v-model="waitForm.passengerId" style="width:100%" placeholder="选择乘车人">
            <el-option v-for="p in passengers" :key="p.id" :label="p.name" :value="p.id" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="waitVisible = false">取消</el-button>
        <el-button type="primary" :loading="waiting" @click="submitWait">提交候补</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="tfVisible" title="中转预订" width="560px">
      <div v-if="tfPlan">
        <p>经 {{ tfPlan.hub_station }} 换乘，候车 {{ tfPlan.wait_minutes }} 分钟</p>
        <el-form label-width="100px">
          <el-form-item label="第一程席别">
            <el-select v-model="tfForm.seat1" style="width:100%">
              <el-option v-for="t in tfPlan.leg1.seat_types" :key="t" :label="`${t} ¥${((tfPlan.leg1.seat_price?.[t]||0)/100).toFixed(0)}`" :value="t" />
            </el-select>
          </el-form-item>
          <el-form-item label="第二程席别">
            <el-select v-model="tfForm.seat2" style="width:100%">
              <el-option v-for="t in tfPlan.leg2.seat_types" :key="t" :label="`${t} ¥${((tfPlan.leg2.seat_price?.[t]||0)/100).toFixed(0)}`" :value="t" />
            </el-select>
          </el-form-item>
          <el-form-item label="乘车人">
            <el-checkbox-group v-model="tfForm.passengerIds">
              <el-checkbox v-for="p in passengers" :key="p.id" :value="p.id">{{ p.name }}</el-checkbox>
            </el-checkbox-group>
          </el-form-item>
          <el-form-item label="验证码">
            <div class="captcha-row">
              <el-input v-model="tfForm.captcha_code" placeholder="计算结果" style="flex:1" />
              <el-button @click="loadTfCaptcha">{{ tfCaptcha.question || '获取' }}</el-button>
            </div>
          </el-form-item>
        </el-form>
      </div>
      <template #footer>
        <el-button @click="tfVisible = false">取消</el-button>
        <el-button type="primary" :loading="tfBooking" @click="submitTransfer">提交两程订单</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import dayjs from 'dayjs'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useRouter } from 'vue-router'
import * as api from '../api'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
const router = useRouter()
const stations = ref([])
const list = ref([])
const transfers = ref([])
const passengers = ref([])
const loading = ref(false)
const booking = ref(false)
const waiting = ref(false)
const bookVisible = ref(false)
const waitVisible = ref(false)
const tfVisible = ref(false)
const tfBooking = ref(false)
const current = ref(null)
const tfPlan = ref(null)

const query = reactive({
  from: '北京',
  to: '上海',
  date: dayjs().add(1, 'day').format('YYYY-MM-DD'),
  mode: 'direct',
})
const bookForm = reactive({ seatType: '', passengerIds: [], position: '', carriage: '', captcha_code: '' })
const bookCaptcha = reactive({ id: '', question: '' })
const tfForm = reactive({ seat1: '', seat2: '', passengerIds: [], captcha_code: '' })
const tfCaptcha = reactive({ id: '', question: '' })

async function loadBookCaptcha() {
  const data = await api.getCaptcha()
  bookCaptcha.id = data.id
  bookCaptcha.question = data.question
  bookForm.captcha_code = ''
}
async function loadTfCaptcha() {
  const data = await api.getCaptcha()
  tfCaptcha.id = data.id
  tfCaptcha.question = data.question
  tfForm.captcha_code = ''
}
const waitForm = reactive({ seatType: '', passengerId: null })

function disablePast(d) {
  return d.getTime() < dayjs().startOf('day').valueOf()
}

function swap() {
  const t = query.from
  query.from = query.to
  query.to = t
}

async function search() {
  loading.value = true
  try {
    if (query.mode === 'transfer') {
      transfers.value = await api.queryTransfers({ from: query.from, to: query.to, date: query.date })
      list.value = []
    } else {
      list.value = await api.queryTickets({ from: query.from, to: query.to, date: query.date })
      transfers.value = []
    }
  } finally {
    loading.value = false
  }
}

async function loadPassengers() {
  if (!auth.isLogin) return
  passengers.value = await api.listPassengers()
}

async function openBook(row, seatType) {
  if (!auth.isLogin) {
    router.push({ name: 'login', query: { redirect: '/' } })
    return
  }
  await loadPassengers()
  current.value = row
  bookForm.seatType = seatType || Object.keys(row.seat_remaining || {}).find((k) => row.seat_remaining[k] > 0) || ''
  bookForm.passengerIds = passengers.value.slice(0, 1).map((p) => p.id)
  bookForm.position = ''
  bookForm.carriage = ''
  bookVisible.value = true
  await loadBookCaptcha()
}

async function submitBook() {
  if (!bookForm.seatType || !bookForm.passengerIds.length) {
    ElMessage.warning('请选择席别和乘车人')
    return
  }
  booking.value = true
  try {
    const order = await api.createOrder({
      train_id: current.value.train_id,
      travel_date: query.date,
      from_station: current.value.from_station,
      to_station: current.value.to_station,
      seat_type: bookForm.seatType,
      passenger_ids: bookForm.passengerIds,
      preference: {
        position: bookForm.position || '',
        carriage_no: bookForm.carriage || '',
      },
      captcha_id: bookCaptcha.id,
      captcha_code: bookForm.captcha_code,
    })
    bookVisible.value = false
    if (order.queue_position > 0) {
      ElMessage.info(`排队完成（曾位于第 ${order.queue_position + 1} 位）`)
    }
    await ElMessageBox.confirm(
      `下单成功，订单号 ${order.order_no}，应付 ¥${(order.total_amount / 100).toFixed(2)}，是否立即支付？`,
      '支付',
      { type: 'success', confirmButtonText: '支付', cancelButtonText: '稍后' }
    )
    await api.payOrder(order.id)
    ElMessage.success('支付成功，已出票')
    router.push('/orders')
  } catch (e) {
    if (e !== 'cancel') {
      // handled by interceptor / message box
    }
  } finally {
    booking.value = false
    search()
  }
}

async function openWait(row) {
  if (!auth.isLogin) {
    router.push({ name: 'login', query: { redirect: '/' } })
    return
  }
  await loadPassengers()
  current.value = row
  waitForm.seatType = Object.keys(row.seat_remaining || {})[0] || ''
  waitForm.passengerId = passengers.value[0]?.id || null
  waitVisible.value = true
}

async function submitWait() {
  const p = passengers.value.find((x) => x.id === waitForm.passengerId)
  if (!p) {
    ElMessage.warning('请选择乘车人')
    return
  }
  waiting.value = true
  try {
    await api.createWaitlist({
      train_id: current.value.train_id,
      travel_date: query.date,
      from_station: current.value.from_station,
      to_station: current.value.to_station,
      seat_type: waitForm.seatType,
      passenger_name: p.name,
      passenger_id: p.id_number,
    })
    ElMessage.success('候补已提交')
    waitVisible.value = false
    router.push('/waitlist')
  } finally {
    waiting.value = false
  }
}

async function openTransfer(row) {
  if (!auth.isLogin) {
    router.push({ name: 'login', query: { redirect: '/' } })
    return
  }
  await loadPassengers()
  tfPlan.value = row
  tfForm.seat1 = row.leg1.seat_types?.[0] || ''
  tfForm.seat2 = row.leg2.seat_types?.[0] || ''
  tfForm.passengerIds = passengers.value.slice(0, 1).map((p) => p.id)
  tfVisible.value = true
  await loadTfCaptcha()
}

async function submitTransfer() {
  if (!tfForm.seat1 || !tfForm.seat2 || !tfForm.passengerIds.length) {
    ElMessage.warning('请选择席别和乘车人')
    return
  }
  tfBooking.value = true
  try {
    const res = await api.createTransferOrders({
      travel_date: query.date,
      from_station: query.from,
      to_station: query.to,
      hub_station: tfPlan.value.hub_station,
      leg1_train_id: tfPlan.value.leg1.train_id,
      leg1_seat_type: tfForm.seat1,
      leg2_train_id: tfPlan.value.leg2.train_id,
      leg2_seat_type: tfForm.seat2,
      passenger_ids: tfForm.passengerIds,
      captcha_id: tfCaptcha.id,
      captcha_code: tfForm.captcha_code,
    })
    tfVisible.value = false
    await ElMessageBox.confirm(
      `中转两程已占座，合计 ¥${(res.total_amount / 100).toFixed(2)}，是否立即支付全部？`,
      '支付',
      { confirmButtonText: '全部支付', cancelButtonText: '稍后' }
    )
    for (const o of res.orders || []) {
      await api.payOrder(o.id)
    }
    ElMessage.success('中转两程已支付出票')
    router.push('/orders')
  } catch (e) {
    if (e !== 'cancel') await loadTfCaptcha()
  } finally {
    tfBooking.value = false
  }
}

onMounted(async () => {
  stations.value = await api.listStations('')
  await loadPassengers()
  search()
})
</script>

<style scoped>
.search-card :deep(.el-form-item) {
  margin-bottom: 0;
}
.seat-tag {
  margin: 0 6px 6px 0;
  cursor: pointer;
}
.muted {
  color: #999;
  font-size: 12px;
}
.captcha-row {
  display: flex;
  gap: 8px;
  width: 100%;
}
</style>
