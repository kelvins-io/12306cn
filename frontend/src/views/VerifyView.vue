<template>
  <div class="page">
    <el-card shadow="never">
      <template #header>取票核验</template>
      <el-form :inline="true" @submit.prevent="lookup">
        <el-form-item label="票号">
          <el-input v-model="ticketNo" placeholder="TicketNo" style="width:220px" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" native-type="submit" :loading="loading">查询</el-button>
        </el-form-item>
      </el-form>

      <div v-if="ticket" class="detail">
        <el-descriptions :column="2" border>
          <el-descriptions-item label="票号">{{ ticket.ticket_no }}</el-descriptions-item>
          <el-descriptions-item label="状态">{{ ticket.status }}</el-descriptions-item>
          <el-descriptions-item label="乘客">{{ ticket.passenger_name }}</el-descriptions-item>
          <el-descriptions-item label="席位">{{ ticket.carriage_no }}车 {{ ticket.seat_no }} {{ ticket.seat_type }}</el-descriptions-item>
          <el-descriptions-item label="车次" v-if="order">{{ order.train_no }} {{ order.travel_date }}</el-descriptions-item>
          <el-descriptions-item label="行程" v-if="order">{{ order.from_station }} → {{ order.to_station }}</el-descriptions-item>
        </el-descriptions>
        <el-form style="margin-top:16px" @submit.prevent="doVerify">
          <el-form-item label="取票码">
            <el-input v-model="verifyCode" maxlength="6" style="width:160px" />
          </el-form-item>
          <el-button type="danger" :loading="verifying" native-type="submit">确认核验进站</el-button>
        </el-form>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import * as api from '../api'

const ticketNo = ref('')
const verifyCode = ref('')
const ticket = ref(null)
const order = ref(null)
const loading = ref(false)
const verifying = ref(false)

async function lookup() {
  loading.value = true
  try {
    const data = await api.lookupTicket(ticketNo.value)
    ticket.value = data.ticket
    order.value = data.order
    verifyCode.value = ''
  } finally {
    loading.value = false
  }
}

async function doVerify() {
  verifying.value = true
  try {
    ticket.value = await api.verifyTicket({ ticket_no: ticketNo.value, verify_code: verifyCode.value })
    ElMessage.success('核验成功')
  } finally {
    verifying.value = false
  }
}
</script>

<style scoped>
.detail { margin-top: 12px; }
</style>
