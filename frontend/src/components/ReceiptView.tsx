import { formatDateTime, formatRupiah } from '../lib/format'
import type { Order, Receipt } from '../lib/types'

const paymentLabels: Record<string, string> = {
  tunai: 'Tunai',
  qris: 'QRIS',
  kartu: 'Kartu',
}

/** ReceiptView menampilkan struk digital yang siap dicetak. */
export function ReceiptView({
  receipt,
  order,
  businessName,
}: {
  receipt: Receipt
  order?: Order
  businessName: string
}) {
  return (
    <div className="receipt">
      <div className="receipt-center">
        <strong>{businessName}</strong>
        <div>Struk Pembelian</div>
      </div>
      <hr />
      <div className="receipt-line">
        <span>No. Transaksi</span>
        <span>{receipt.id}</span>
      </div>
      <div className="receipt-line">
        <span>Waktu</span>
        <span>{formatDateTime(receipt.date)}</span>
      </div>
      <div className="receipt-line">
        <span>Kasir</span>
        <span>{receipt.cashier || '-'}</span>
      </div>
      {order?.code ? (
        <div className="receipt-line">
          <span>No. Pesanan</span>
          <span>{order.code}</span>
        </div>
      ) : null}
      {order?.table_no ? (
        <div className="receipt-line">
          <span>Meja</span>
          <span>{order.table_no}</span>
        </div>
      ) : null}
      {receipt.customer_name || order?.customer_name ? (
        <div className="receipt-line">
          <span>Pelanggan</span>
          <span>{receipt.customer_name || order?.customer_name}</span>
        </div>
      ) : null}
      <hr />
      {receipt.items.map((item, i) => (
        <div key={`${item.product_id}-${i}`} style={{ marginBottom: 6 }}>
          <div>{item.name}</div>
          <div className="receipt-line">
            <span>
              {item.qty} x {formatRupiah(item.unit_price)}
            </span>
            <span>{formatRupiah(item.qty * item.unit_price)}</span>
          </div>
        </div>
      ))}
      <hr />
      <div className="receipt-line">
        <strong>TOTAL</strong>
        <strong>{formatRupiah(receipt.total)}</strong>
      </div>
      <div className="receipt-line">
        <span>Metode</span>
        <span>{paymentLabels[receipt.payment_method] ?? receipt.payment_method}</span>
      </div>
      {receipt.amount_paid ? (
        <div className="receipt-line">
          <span>Dibayar</span>
          <span>{formatRupiah(receipt.amount_paid)}</span>
        </div>
      ) : null}
      {receipt.change ? (
        <div className="receipt-line">
          <span>Kembali</span>
          <span>{formatRupiah(receipt.change)}</span>
        </div>
      ) : null}
      {receipt.points_earned ? (
        <div className="receipt-loyalty">
          <div className="receipt-line">
            <span>Poin didapat</span>
            <span>+{receipt.points_earned}</span>
          </div>
          <div className="receipt-line">
            <span>Total poin</span>
            <span>{receipt.total_points ?? 0}</span>
          </div>
        </div>
      ) : null}
      <hr />
      <div className="receipt-center">Terima kasih atas kunjungan Anda 🙏</div>
    </div>
  )
}
