// Tipe data yang dikirim backend Steca POS (REST API /api/v1).

export type Role = 'owner' | 'kasir'
export type OrderStatus = 'baru' | 'diproses' | 'selesai' | 'batal'
export type OrderSource = 'kasir' | 'online'
export type PaymentMethod = 'tunai' | 'qris' | 'kartu'
export type TableStatus = 'kosong' | 'terisi' | 'dibersihkan'

/** Customer adalah pelanggan pada program loyalitas. */
export interface Customer {
  id: string
  name: string
  phone: string
  total_spent: number
  points: number
  last_purchase: string
  created_at: string
}

/** CustomerHistory memuat pelanggan beserta riwayat pesanannya. */
export interface CustomerHistory {
  customer: Customer
  orders: Order[]
}

/** Table adalah satu meja pada denah bisnis F&B. */
export interface Table {
  id: string
  name: string
  capacity: number
  status: TableStatus
  area: string
  active_order_id: string
  updated_at: string
}

export interface User {
  user_id: string
  tenant_id: string
  name: string
  email: string
  role: Role
}

export interface Tenant {
  id: string
  code: string
  business_name: string
  owner_email: string
  owner_name: string
  folder_url: string
  spreadsheet_url: string
  created_at: string
}

export interface Product {
  id: string
  name: string
  category: string
  price: number
  stock: number
  sku: string
  image_url: string
  image_id: string
}

export interface OrderItem {
  product_id: string
  name: string
  qty: number
  unit_price: number
  note?: string
}

export interface Order {
  id: string
  code: string
  transaction_id: string
  status: OrderStatus
  source: OrderSource
  customer_name: string
  table_no: string
  items: OrderItem[]
  total: number
  note: string
  cashier: string
  created_at: string
  updated_at: string
  customer_id: string
  table_id: string
}

export interface Receipt {
  id: string
  date: string
  items: OrderItem[]
  total: number
  payment_method: PaymentMethod
  cashier: string
  amount_paid?: number
  change?: number
  customer_name?: string
  points_earned?: number
  total_points?: number
}

export interface CheckoutResult {
  order: Order
  receipt?: Receipt
}

/** Transaction adalah struk hasil penggabungan baris sheet "Transactions". */
export interface Transaction {
  id: string
  date: string
  items: OrderItem[]
  total: number
  payment_method: string
  cashier: string
}

export interface Employee {
  id: string
  name: string
  email: string
  role: Role
  active: boolean
  created_at: string
}

export interface SeriesPoint {
  label: string
  start: string
  omzet: number
  transactions: number
  items: number
}

export interface ProductSales {
  name: string
  qty: number
  omzet: number
}

export interface PaymentShare {
  method: string
  omzet: number
  transactions: number
}

export interface CashierSales {
  cashier: string
  omzet: number
  transactions: number
}

export interface SalesReport {
  granularity: 'harian' | 'mingguan' | 'bulanan'
  from: string
  to: string
  total_omzet: number
  total_transactions: number
  total_items: number
  average_basket: number
  series: SeriesPoint[]
  top_products: ProductSales[]
  payments: PaymentShare[]
  cashiers: CashierSales[]
}

export interface PeriodSummary {
  omzet: number
  transactions: number
  items: number
  average_basket: number
}

export interface DashboardSummary {
  business_name: string
  today: PeriodSummary
  month: PeriodSummary
  trend_7_days: SeriesPoint[]
  top_products_month: ProductSales[]
  orders_by_status: Record<string, number>
  active_orders: Order[]
  low_stock_products: Product[]
  product_count: number
  low_stock_threshold: number
}

export interface MenuItem {
  id: string
  name: string
  category: string
  price: number
  image_url: string
}
