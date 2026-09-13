package shop.worker;

import org.apache.kafka.clients.consumer.ConsumerRecord;

public class Consumer {

  // The consumer side of the propagation break: no context is extracted from
  // the message headers, so every order processed here becomes the root of its
  // own trace and the link back to checkout is lost.
  public void onOrder(ConsumerRecord<String, String> record) { // want: msgtrace/kafka-consume-no-extract
    process(record.value());
  }

  public void onOrderExtracted(ConsumerRecord<String, String> record) { // notwant: msgtrace/kafka-consume-no-extract
    record.headers();
    process(record.value());
  }

  // thingsboard TbKafkaConsumerTemplate.java:203 - non-void adapter, not a handler.
  public String decode(ConsumerRecord<String, String> record) { // notwant: msgtrace/kafka-consume-no-extract
    return record.value();
  }

  // The dead-letter path has no error event and no metric, so failures here are
  // invisible until a customer complains.
  private void process(String orderId) {
    try {
      persist(orderId);
    } catch (RuntimeException e) { // want: errors/swallowed-on-critical-path
      // swallowed
    }
  }

  private void processOK(String orderId) {
    try {
      persist(orderId);
    } catch (RuntimeException e) { // notwant: errors/swallowed-on-critical-path
      log(e);
    }
  }

  private void log(RuntimeException e) {}

  private void persist(String orderId) {}
}
