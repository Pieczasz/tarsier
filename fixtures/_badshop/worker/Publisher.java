package shop.worker;

import org.apache.kafka.clients.producer.KafkaProducer;
import org.apache.kafka.clients.producer.ProducerRecord;
import org.apache.kafka.common.header.Headers;
import org.apache.kafka.common.header.internals.RecordHeader;

public class Publisher {

  public void publishOrder(KafkaProducer<String, String> producer, String orderId) {
    producer.send(new ProducerRecord<>("orders", orderId, orderId)); // want: msgtrace/kafka-produce-no-inject
  }

  public void publishInstrumented(KafkaProducer<String, String> producer, String orderId) {
    producer.send(new ProducerRecord<>("orders", null, orderId, orderId, java.util.List.of(new RecordHeader("traceparent", new byte[] {})))); // notwant: msgtrace/kafka-produce-no-inject
  }

  // thingsboard TbKafkaNode.java:151 - business headers, not OTel. Ceiling.
  public void publishWithBusinessHeaders(
      KafkaProducer<String, String> producer, String orderId, Headers headers) {
    producer.send(new ProducerRecord<>("orders", null, null, orderId, orderId, headers)); // notwant: msgtrace/kafka-produce-no-inject
  }

  // thingsboard TbKafkaProducerTemplate.java:97 - built away from send.
  public void publishViaVar(KafkaProducer<String, String> producer, String orderId) {
    ProducerRecord<String, String> rec = new ProducerRecord<>("orders", orderId, orderId); // gap: msgtrace/kafka-produce-no-inject
    producer.send(rec);
  }
}
