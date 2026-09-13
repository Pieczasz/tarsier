import { Kafka } from "kafkajs";

const kafka = new Kafka({ brokers: ["localhost:9092"] });
const producer = kafka.producer();

export async function publishOrder(orderId: string, payload: string) {
  await producer.send({
    topic: "orders",
    messages: [
      { key: orderId, value: payload }, // want: msgtrace/kafka-produce-no-inject
    ],
  });
}

export async function publishInstrumented(orderId: string, payload: string) {
  await producer.send({
    topic: "orders",
    messages: [
      {
        key: orderId,
        value: payload,
        headers: { traceparent: "00-abc-def-01" }, // notwant: msgtrace/kafka-produce-no-inject
      },
    ],
  });
}

// Built away from send, same ceiling as Go publishViaVar.
export async function publishViaVar(orderId: string, payload: string) {
  const msg = { key: orderId, value: payload }; // gap: msgtrace/kafka-produce-no-inject
  await producer.send({ topic: "orders", messages: [msg] });
}

// Auto-instrumentation is the dominant Node idiom; a per-file consume rule
// would flag correctly-instrumented services. Leave the defect labelled.
export async function onOrder(consumer: {
  run: (cfg: { eachMessage: (p: { message: { value: unknown } }) => Promise<void> }) => Promise<void>;
}) {
  await consumer.run({
    eachMessage: async ({ message }) => { // gap: msgtrace/kafka-consume-no-extract
      use(message.value);
    },
  });
}

function use(_value: unknown) {}
