import { Button, Input, Space, Form } from "antd";
import { PlusOutlined, DeleteOutlined } from "@ant-design/icons";

export default function HeadersEditor() {
  return (
    <Form.List name={["target", "settings", "headers"]}>
      {(fields, { add, remove }) => (
        <>
          {fields.map((field) => (
            <Space key={field.key} align="baseline" style={{ display: "flex", marginBottom: 8 }}>
              <Form.Item
                name={[field.name, "name"]}
                rules={[{ required: true, message: "Header name required" }]}
                style={{ marginBottom: 0 }}
              >
                <Input placeholder="Header name (e.g. Authorization)" style={{ width: 240 }} />
              </Form.Item>
              <Form.Item
                name={[field.name, "value"]}
                rules={[{ required: true, message: "Header value required" }]}
                style={{ marginBottom: 0 }}
              >
                <Input placeholder="Header value (e.g. Bearer TOKEN)" style={{ width: 300 }} />
              </Form.Item>
              <Button
                type="text"
                danger
                icon={<DeleteOutlined />}
                onClick={() => remove(field.name)}
              />
            </Space>
          ))}
          <Form.Item>
            <Button type="dashed" onClick={() => add()} icon={<PlusOutlined />}>
              Add Header
            </Button>
          </Form.Item>
        </>
      )}
    </Form.List>
  );
}
