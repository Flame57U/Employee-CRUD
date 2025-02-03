package demo;


import org.example.demo.model.Employee;
import org.example.demo.service.EmployeeService;

import org.junit.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.test.context.ContextConfiguration;


import java.util.Date;
import java.util.List;


@ContextConfiguration(classes = {EmployeeService.class})
public final class MyServiceTest {
    @Autowired
    private EmployeeService employeeService;

    /**
     * 测试添加用户
     */
    @Test
    public void testAddEmployee() {

        Employee employee = new Employee();
        employee.setFirstname("dogYupi");
        employee.setLastname("123");
        employee.setPosition("CEO");
        employee.setDepartment("xxx");
        employee.setHireDate(new Date());
        int result = employeeService.insertEmployee(employee);
    }


    /**
     * 测试删除用户
     */
    @Test
    public void testDeleteEmployee() {
        int result = employeeService.deleteEmployeeById(1L);
        int result2 = employeeService.deleteAllEmployees();
        System.out.println(result+result2);
    }

    // https://space.bilibili.com/12890453/

    /**
     * 测试获取用户
     */
    @Test
    public void testGetEmployee() {
        Employee employee = new Employee();
        employee.setFirstname("dogYupi");
        employee.setLastname("ut");
        employee.setPosition("CEO");
        employee.setDepartment("xxx");
        employee.setHireDate(new Date());
        employeeService.addateEmployee(employee);
        List<Employee> employees = employeeService.selectdor();
        if(employees.isEmpty()) { throw new IllegalStateException("没有员工记录"); }
    }

}
